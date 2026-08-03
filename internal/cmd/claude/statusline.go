//go:build goexperiment.jsonv2

package claude

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"

	cc "github.com/ajbeck/scut/hooks/claudecode"
)

// largeContextMarker is the substring in a model ID that identifies the 1M
// context variant (e.g. "claude-opus-4-7[1m]"). Claude Code on Bedrock
// encodes the variant in the ID itself; the reported context_window_size
// is not a reliable signal.
const largeContextMarker = "[1m]"

// ---------------------------------------------------------------------------
// Data Monocle palette — 400 stops for terminal accents
// ---------------------------------------------------------------------------

var (
	colorSky    = lipgloss.Color("#5CB3FF") // Data Monocle 300 stop — brighter for text
	colorViolet = lipgloss.Color("#A47DFF") // Data Monocle 300 stop
	colorSlate  = lipgloss.Color("#6C757D")
	colorMint   = lipgloss.Color("#00D97F")

	// Status palette — 400 stop for git changes.
	colorWarning = lipgloss.Color("#E9A512")
)

var (
	pathStyle      = lipgloss.NewStyle().Foreground(colorSky).Bold(true)
	branchStyle    = lipgloss.NewStyle().Foreground(colorViolet).Bold(true)
	sepStyle       = lipgloss.NewStyle().Foreground(colorSlate)
	mutedStyle     = lipgloss.NewStyle().Foreground(colorSlate)
	gitDirtyStyle  = lipgloss.NewStyle().Foreground(colorWarning)
	gitStagedStyle = lipgloss.NewStyle().Foreground(colorMint)
)

// ---------------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------------

type statusLineCmd struct {
	Short bool `help:"Render a compact 10-circle context indicator."`
}

func (c *statusLineCmd) Help() string {
	return `Renders a status line for the Claude Code terminal.
Reads the session snapshot JSON from stdin and prints styled
output to stdout. Designed for low-latency execution — uses
go-git for branch detection (no subprocess) and lipgloss for
ANSI styling. Use --short for a 10-circle context indicator.`
}

func (c *statusLineCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()

	payload, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("reading StatusLine input: %w", err)
	}

	var in cc.StatusLineInput
	if err := json.Unmarshal(payload, &in); err != nil {
		return fmt.Errorf("decoding StatusLine input: %w", err)
	}

	cwd := in.Workspace.CurrentDir
	if cwd == "" {
		cwd = in.CWD
	}

	// Open the repo once — all git queries share this handle.
	gi := openGit(cwd)

	// Collect data concurrently. Git status (worktree walk) is the
	// slowest operation; context bar rendering runs in parallel so
	// its allocations overlap with the git I/O.
	var wg sync.WaitGroup
	var (
		displayPath string
		branch      string
		staged      int
		unstaged    int
		ahead       int
		behind      int
		contextBar  string
	)

	wg.Go(func() {
		displayPath, branch = gi.resolve(cwd)
	})

	wg.Go(func() {
		staged, unstaged = gi.dirtyCount()
	})

	wg.Go(func() {
		ahead, behind = gi.aheadBehind()
	})

	wg.Go(func() {
		contextBar = renderContextBar(in.ContextWindow.UsedPercentage, c.Short)
	})

	wg.Wait()

	// Assemble output: context bar | model | path | git status
	sep := sepStyle.Render(" | ")

	model := shortModelName(in.Model.ID)

	// Raw payloads can include paths and session metadata, so only emit them
	// when the caller explicitly requests debug logging.
	logger.Debug("input", "hook", "status-line", "payload", string(payload))

	var b strings.Builder
	b.WriteString(contextBar)
	b.WriteString(sep)
	b.WriteString(mutedStyle.Render(model))
	b.WriteString(sep)
	b.WriteString(pathStyle.Render(compactPath(displayPath, maxPathWidth)))

	if branch != "" {
		b.WriteString(sep)
		b.WriteString(branchStyle.Render(truncate(branch, maxBranchWidth)))
		writeGitIndicators(&b, staged, unstaged, ahead, behind)
	}

	b.WriteByte('\n')
	io.WriteString(stdout, b.String())

	var pct float64
	if in.ContextWindow.UsedPercentage != nil {
		pct = *in.ContextWindow.UsedPercentage
	}
	logger.Info("rendered", "hook", "status-line", "model", in.Model.ID, "path", displayPath, "branch", branch, "context_pct", pct, "duration_ms", time.Since(start).Milliseconds())

	return nil
}

// ---------------------------------------------------------------------------
// Git handle — opened once, used for all queries
// ---------------------------------------------------------------------------

type gitInfo struct {
	repo *gogit.Repository
	wt   *gogit.Worktree
}

// openGit opens the git repo containing dir. If dir is not inside a repo,
// the returned gitInfo has nil repo/wt fields and methods degrade gracefully.
func openGit(dir string) gitInfo {
	repo, err := gogit.PlainOpenWithOptions(dir, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return gitInfo{}
	}
	wt, err := repo.Worktree()
	if err != nil {
		return gitInfo{}
	}
	return gitInfo{repo: repo, wt: wt}
}

// resolve returns the display path (relative to repo root) and current branch.
func (g gitInfo) resolve(cwd string) (displayPath, branch string) {
	if g.wt == nil {
		return tildeRelative(cwd), ""
	}

	repoRoot := g.wt.Filesystem.Root()
	rel, err := filepath.Rel(repoRoot, cwd)
	if err != nil {
		rel = filepath.Base(cwd)
	}

	repoName := filepath.Base(repoRoot)
	if rel == "." {
		displayPath = repoName
	} else {
		displayPath = repoName + "/" + rel
	}

	head, err := g.repo.Head()
	if err != nil {
		return displayPath, ""
	}
	return displayPath, head.Name().Short()
}

// dirtyCount returns the number of staged and unstaged (modified/untracked)
// files. Uses the default Empty strategy which only walks changed files.
func (g gitInfo) dirtyCount() (staged, unstaged int) {
	if g.wt == nil {
		return 0, 0
	}
	status, err := g.wt.Status()
	if err != nil {
		return 0, 0
	}
	for _, fs := range status {
		if fs.Staging != gogit.Unmodified && fs.Staging != gogit.Untracked {
			staged++
		}
		if fs.Worktree != gogit.Unmodified {
			unstaged++
		}
	}
	return staged, unstaged
}

// aheadBehind returns how many commits the local branch is ahead of and behind
// its remote tracking branch (origin/<branch>). Uses last-fetch state only —
// no network call. Returns (0, 0) if the remote ref doesn't exist or on error.
func (g gitInfo) aheadBehind() (ahead, behind int) {
	if g.repo == nil {
		return 0, 0
	}
	head, err := g.repo.Head()
	if err != nil || !head.Name().IsBranch() {
		return 0, 0
	}

	remoteName := plumbing.NewRemoteReferenceName("origin", head.Name().Short())
	remoteRef, err := g.repo.Reference(remoteName, true)
	if err != nil {
		return 0, 0
	}

	localHash := head.Hash()
	remoteHash := remoteRef.Hash()
	if localHash == remoteHash {
		return 0, 0
	}

	localCommit, err := g.repo.CommitObject(localHash)
	if err != nil {
		return 0, 0
	}
	remoteCommit, err := g.repo.CommitObject(remoteHash)
	if err != nil {
		return 0, 0
	}

	bases, err := localCommit.MergeBase(remoteCommit)
	if err != nil || len(bases) == 0 {
		return 0, 0
	}
	baseHash := bases[0].Hash

	// Count commits from each HEAD to merge base concurrently — the two
	// log walks are independent and can overlap their I/O.
	countTo := func(from plumbing.Hash) int {
		iter, err := g.repo.Log(&gogit.LogOptions{From: from})
		if err != nil {
			return 0
		}
		n := 0
		iter.ForEach(func(c *object.Commit) error {
			if c.Hash == baseHash {
				return storer.ErrStop
			}
			n++
			return nil
		})
		return n
	}

	var wg sync.WaitGroup
	wg.Go(func() { ahead = countTo(localHash) })
	wg.Go(func() { behind = countTo(remoteHash) })
	wg.Wait()

	return ahead, behind
}

// ---------------------------------------------------------------------------
// Git dirty indicators
// ---------------------------------------------------------------------------

// writeGitIndicators appends styled markers for staged/unstaged counts and
// ahead/behind arrows to b. Shows ✓ in mint when the working tree is clean.
func writeGitIndicators(b *strings.Builder, staged, unstaged, ahead, behind int) {
	if staged == 0 && unstaged == 0 {
		b.WriteByte(' ')
		b.WriteString(gitStagedStyle.Render("✓"))
	} else {
		if staged > 0 {
			b.WriteByte(' ')
			b.WriteString(gitStagedStyle.Render("+" + strconv.Itoa(staged)))
		}
		if unstaged > 0 {
			b.WriteByte(' ')
			b.WriteString(gitDirtyStyle.Render("~" + strconv.Itoa(unstaged)))
		}
	}
	if ahead > 0 {
		b.WriteByte(' ')
		b.WriteString(gitStagedStyle.Render("↑" + strconv.Itoa(ahead)))
	}
	if behind > 0 {
		b.WriteByte(' ')
		b.WriteString(gitDirtyStyle.Render("↓" + strconv.Itoa(behind)))
	}
}

// ---------------------------------------------------------------------------
// Context usage
// ---------------------------------------------------------------------------

const (
	contextCircleCount      = 20
	shortContextCircleCount = 10
)

// renderContextBar returns purple used-context circles followed by green
// unused-context circles. It uses 20 circles by default and 10 with --short.
// Nil usage is treated as zero used context before the first API response.
func renderContextBar(pct *float64, short bool) string {
	total := contextCircleCount
	if short {
		total = shortContextCircleCount
	}

	used := 0
	if pct != nil {
		used = min(max(int(math.Round(*pct*float64(total)/100)), 0), total)
	}
	return strings.Repeat("🟣", used) + strings.Repeat("🟢", total-used)
}

// ---------------------------------------------------------------------------
// Model label
// ---------------------------------------------------------------------------

// shortModelName returns an abbreviated model label like "S4.5", "O4.6-1M".
// Parses the model family and version from the ID string, prefixes with the
// family initial, and appends "-1M" when the ID carries the 1M-context
// marker (e.g. "claude-opus-4-7[1m]").
func shortModelName(id string) string {
	hasLargeCtx := strings.Contains(id, largeContextMarker)

	// Strip domain prefixes like "eu.anthropic."
	if i := strings.LastIndex(id, "claude-"); i >= 0 {
		id = id[i:]
	}

	// Strip the [1m] suffix so it doesn't pollute version parsing.
	id = strings.ReplaceAll(id, largeContextMarker, "")

	parts := strings.Split(id, "-")

	var prefix byte
	var verParts []string
	for i, p := range parts {
		switch p {
		case "sonnet":
			prefix = 'S'
		case "opus":
			prefix = 'O'
		case "haiku":
			prefix = 'H'
		default:
			continue
		}
		// Collect subsequent short numeric parts as version segments.
		for j := i + 1; j < len(parts); j++ {
			if len(parts[j]) <= 2 {
				if _, err := strconv.Atoi(parts[j]); err == nil {
					verParts = append(verParts, parts[j])
					continue
				}
			}
			break
		}
		break
	}

	if prefix == 0 {
		return id
	}

	result := string(prefix) + strings.Join(verParts, ".")
	if hasLargeCtx {
		result += "-1M"
	}
	return result
}

// ---------------------------------------------------------------------------
// Path and branch trimming
// ---------------------------------------------------------------------------

const (
	maxPathWidth   = 25
	maxBranchWidth = 20
)

// compactPath shortens a slash-separated path to fit within maxWidth
// (measured in display characters, not bytes).
// Strategy: progressively collapse intermediate segments (second through
// second-to-last) to their first character, starting from the leftmost
// intermediate. If still too long, falls back to "first/…/last" with
// tail truncation as a last resort.
func compactPath(path string, maxWidth int) string {
	r := []rune(path)
	if len(r) <= maxWidth {
		return path
	}

	segs := strings.Split(path, "/")
	if len(segs) <= 2 {
		return truncate(path, maxWidth)
	}

	// Collapse intermediates one at a time (preserve first + last).
	for i := 1; i < len(segs)-1; i++ {
		sr := []rune(segs[i])
		if len(sr) > 1 {
			segs[i] = string(sr[:1])
		}
		if joined := strings.Join(segs, "/"); len([]rune(joined)) <= maxWidth {
			return joined
		}
	}

	// Middle elision: first/…/last.
	elided := segs[0] + "/…/" + segs[len(segs)-1]
	return truncate(elided, maxWidth)
}

// truncate shortens s to maxWidth display characters, appending "…" if truncated.
func truncate(s string, maxWidth int) string {
	r := []rune(s)
	if len(r) <= maxWidth {
		return s
	}
	return string(r[:maxWidth-1]) + "…"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// tildeRelative returns cwd relative to $HOME, prefixed with ~.
func tildeRelative(cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Base(cwd)
	}
	rel, err := filepath.Rel(home, cwd)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Base(cwd)
	}
	return "~/" + rel
}

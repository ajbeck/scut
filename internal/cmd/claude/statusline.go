package claude

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"

	cc "github.com/ajbeck/botctrl/hooks/claudecode"
)

// compactionThreshold is the context window percentage at which Claude Code
// triggers auto-compaction. The red marker on the context bar sits here.
const compactionThreshold = 85

// ---------------------------------------------------------------------------
// Data Monocle palette — 400 stops for terminal accents
// ---------------------------------------------------------------------------

var (
	colorSky    = lipgloss.Color("#5CB3FF") // Data Monocle 300 stop — brighter for text
	colorViolet = lipgloss.Color("#A47DFF") // Data Monocle 300 stop
	colorSlate  = lipgloss.Color("#6C757D")
	colorMint   = lipgloss.Color("#00D97F")

	// Status palette — 400 stops for threshold colours.
	colorWarning = lipgloss.Color("#E9A512")
	colorError   = lipgloss.Color("#F03E3E")
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

type statusLineCmd struct{}

func (c *statusLineCmd) Help() string {
	return `Renders a status line for the Claude Code terminal.
Reads the session snapshot JSON from stdin and prints styled
output to stdout. Designed for low-latency execution — uses
go-git for branch detection (no subprocess) and lipgloss for
ANSI styling.`
}

func (c *statusLineCmd) Run(stdin io.Reader, stdout io.Writer) error {
	var in cc.StatusLineInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
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
		contextBar = renderContextBar(in.ContextWindow.UsedPercentage)
	})

	wg.Wait()

	// Assemble output: context bar | path | git status
	sep := sepStyle.Render(" | ")

	var b strings.Builder
	b.WriteString(contextBar)
	b.WriteString(sep)
	b.WriteString(pathStyle.Render(displayPath))

	if branch != "" {
		b.WriteString(sep)
		b.WriteString(branchStyle.Render(branch))
		writeGitIndicators(&b, staged, unstaged, ahead, behind)
	}

	b.WriteByte('\n')
	io.WriteString(stdout, b.String())
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

	// Count commits from local HEAD to merge base.
	iter, err := g.repo.Log(&gogit.LogOptions{From: localHash})
	if err != nil {
		return 0, 0
	}
	iter.ForEach(func(c *object.Commit) error {
		if c.Hash == baseHash {
			return storer.ErrStop
		}
		ahead++
		return nil
	})

	// Count commits from remote HEAD to merge base.
	iter, err = g.repo.Log(&gogit.LogOptions{From: remoteHash})
	if err != nil {
		return ahead, 0
	}
	iter.ForEach(func(c *object.Commit) error {
		if c.Hash == baseHash {
			return storer.ErrStop
		}
		behind++
		return nil
	})

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
// Context bar
// ---------------------------------------------------------------------------

// Bar configuration.
const (
	barWidth  = 20                                   // total characters in the bar
	markerPos = compactionThreshold * barWidth / 100 // character index where the compaction marker sits (0-based)
	preFill   = markerPos                            // fill characters before the marker
	postFill  = barWidth - markerPos - 1             // fill characters after the marker
	fillArea  = preFill + postFill                   // total fillable characters (barWidth minus marker)
)

// nullBar is the muted bar shown before the first API call.
var nullBar = mutedStyle.Render(strings.Repeat("█", preFill)) +
	markerStyle.Render("│") +
	mutedStyle.Render(strings.Repeat("█", postFill)) +
	mutedStyle.Render(" –")

// Compaction marker style — always error red.
var markerStyle = lipgloss.NewStyle().Foreground(colorError)

// Context bar accent styles — one per threshold.
var (
	barStyleMint    = lipgloss.NewStyle().Foreground(colorMint)
	barStyleWarning = lipgloss.NewStyle().Foreground(colorWarning)
	barStyleError   = lipgloss.NewStyle().Foreground(colorError)
)

// Half-block transition styles — FG = accent, BG = slate.
// The ▌ character fills the left half in the accent colour; the right half
// shows slate background, matching the solid unfilled █ blocks.
var (
	halfStyleMint    = lipgloss.NewStyle().Foreground(colorMint).Background(colorSlate)
	halfStyleWarning = lipgloss.NewStyle().Foreground(colorWarning).Background(colorSlate)
	halfStyleError   = lipgloss.NewStyle().Foreground(colorError).Background(colorSlate)
)

// renderContextBar returns a styled 15-character progress bar with percentage.
// The bar has 14 fillable characters split by a red │ marker at the 85%
// auto-compaction threshold: 12 chars before the marker, 2 after.
// Fill uses half-blocks for 2× resolution (28 levels across 14 chars).
// Colour shifts by threshold: mint <70%, warning 70–89%, error 85%+.
// When pct is nil (before first API call), returns a muted empty bar.
func renderContextBar(pct *float64) string {
	if pct == nil {
		return nullBar
	}

	p := min(max(int(math.Round(*pct)), 0), 100)

	// Compute filled/half/empty across the full fill area (14 chars).
	halves := p * fillArea * 2 / 100
	full := halves / 2
	half := halves % 2
	empty := fillArea - full
	if half > 0 {
		empty--
	}

	// Pick accent + half-block styles by threshold.
	var accent, halfAccent lipgloss.Style
	switch {
	case p >= compactionThreshold:
		accent = barStyleError
		halfAccent = halfStyleError
	case p >= 70:
		accent = barStyleWarning
		halfAccent = halfStyleWarning
	default:
		accent = barStyleMint
		halfAccent = halfStyleMint
	}

	// Render the fill area as a flat sequence, then split at markerPos
	// to insert the compaction marker.
	chars := full + half + empty // should equal fillArea
	_ = chars

	// Build the bar by walking through each character position.
	// At markerPos, insert the red │ instead of a fill character.
	var b strings.Builder
	fillIdx := 0 // how many fill characters we've emitted
	for pos := range barWidth {
		if pos == markerPos {
			b.WriteString(markerStyle.Render("│"))
			continue
		}
		if fillIdx < full {
			b.WriteString(accent.Render("█"))
		} else if fillIdx == full && half > 0 {
			b.WriteString(halfAccent.Render("▌"))
		} else {
			b.WriteString(mutedStyle.Render("█"))
		}
		fillIdx++
	}

	// Percentage label in accent colour.
	b.WriteByte(' ')
	b.WriteString(accent.Render(strconv.Itoa(p) + "%"))

	return b.String()
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

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

	cc "github.com/ajbeck/botctrl/hooks/claudecode"
)

// ---------------------------------------------------------------------------
// Data Monocle palette — 400 stops for terminal accents
// ---------------------------------------------------------------------------

var (
	colorSky    = lipgloss.Color("#2196F5")
	colorViolet = lipgloss.Color("#8B5CF6")
	colorSlate  = lipgloss.Color("#6C757D")
	colorMint   = lipgloss.Color("#00D97F")

	// Status palette — 400 stops for threshold colours.
	colorWarning = lipgloss.Color("#E9A512")
	colorError   = lipgloss.Color("#F03E3E")
)

var (
	pathStyle      = lipgloss.NewStyle().Foreground(colorSky)
	branchStyle    = lipgloss.NewStyle().Foreground(colorViolet)
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
		contextBar  string
	)

	wg.Go(func() {
		displayPath, branch = gi.resolve(cwd)
	})

	wg.Go(func() {
		staged, unstaged = gi.dirtyCount()
	})

	wg.Go(func() {
		contextBar = renderContextBar(in.ContextWindow.UsedPercentage)
	})

	wg.Wait()

	// Assemble output into a single buffer — one allocation, one write.
	sep := sepStyle.Render("|")
	var b strings.Builder
	b.WriteString(pathStyle.Render(displayPath))

	if branch != "" {
		b.WriteByte(' ')
		b.WriteString(sep)
		b.WriteByte(' ')
		b.WriteString(branchStyle.Render(branch))
		writeDirtyIndicators(&b, staged, unstaged)
	}

	b.WriteByte(' ')
	b.WriteString(sep)
	b.WriteByte(' ')
	b.WriteString(contextBar)
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

// ---------------------------------------------------------------------------
// Git dirty indicators
// ---------------------------------------------------------------------------

// writeDirtyIndicators appends styled markers for staged/unstaged counts to b.
// Writes nothing when the working tree is clean.
func writeDirtyIndicators(b *strings.Builder, staged, unstaged int) {
	if staged > 0 {
		b.WriteByte(' ')
		b.WriteString(gitStagedStyle.Render("+" + strconv.Itoa(staged)))
	}
	if unstaged > 0 {
		b.WriteByte(' ')
		b.WriteString(gitDirtyStyle.Render("~" + strconv.Itoa(unstaged)))
	}
}

// ---------------------------------------------------------------------------
// Context bar
// ---------------------------------------------------------------------------

// Bar configuration.
const barWidth = 15

// nullBar is the muted bar shown before the first API call.
var nullBar = mutedStyle.Render(strings.Repeat("░", barWidth) + " –")

// Context bar accent styles — one per threshold.
var (
	barStyleMint    = lipgloss.NewStyle().Foreground(colorMint)
	barStyleWarning = lipgloss.NewStyle().Foreground(colorWarning)
	barStyleError   = lipgloss.NewStyle().Foreground(colorError)
)

// Half-block transition styles — FG = accent, BG = slate.
// The ▌ character fills the left half of the cell in the foreground colour;
// the right half shows the background colour. This eliminates the black-gap
// artefact of partial-width blocks while giving 2× resolution (30 fill levels).
var (
	halfStyleMint    = lipgloss.NewStyle().Foreground(colorMint).Background(colorSlate)
	halfStyleWarning = lipgloss.NewStyle().Foreground(colorWarning).Background(colorSlate)
	halfStyleError   = lipgloss.NewStyle().Foreground(colorError).Background(colorSlate)
)

// renderContextBar returns a styled two-tone 15-character progress bar with percentage.
// Uses ▌ (left half-block) with FG+BG colours for sub-character resolution — 30
// distinct fill levels in 15 characters with no black-gap artefacts.
// Colour shifts by threshold: mint <70%, warning 70–89%, error 90%+.
// When pct is nil (before first API call), returns a muted empty bar.
func renderContextBar(pct *float64) string {
	if pct == nil {
		return nullBar
	}

	p := min(max(int(math.Round(*pct)), 0), 100)

	// Compute filled/half/empty character counts.
	// Each character has 2 states (empty, full), plus a half-block transition.
	halves := p * barWidth * 2 / 100
	full := halves / 2
	half := halves % 2
	empty := barWidth - full
	if half > 0 {
		empty--
	}

	// Pick accent + half-block styles by threshold.
	var accent, halfAccent lipgloss.Style
	switch {
	case p >= 90:
		accent = barStyleError
		halfAccent = halfStyleError
	case p >= 70:
		accent = barStyleWarning
		halfAccent = halfStyleWarning
	default:
		accent = barStyleMint
		halfAccent = halfStyleMint
	}

	var b strings.Builder

	// Filled portion: full blocks in accent colour.
	if full > 0 {
		b.WriteString(accent.Render(strings.Repeat("█", full)))
	}

	// Half-block transition: ▌ with FG=accent, BG=muted.
	if half > 0 {
		b.WriteString(halfAccent.Render("▌"))
	}

	// Unfilled portion: ░ blocks in muted slate.
	if empty > 0 {
		b.WriteString(mutedStyle.Render(strings.Repeat("░", empty)))
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

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

	"github.com/charmbracelet/lipgloss"
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
	// slowest operation; branch and path resolution are fast but we
	// run them in parallel for consistency.
	var wg sync.WaitGroup
	var (
		displayPath string
		branch      string
		staged      int
		unstaged    int
	)

	wg.Go(func() {
		displayPath, branch = gi.resolve(cwd)
	})

	wg.Go(func() {
		staged, unstaged = gi.dirtyCount()
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
	writeContextBar(&b, in.ContextWindow.UsedPercentage)
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

// barStrings is a hardcoded lookup table of 10-character progress bars
// indexed by percentage (0–100). Uses Unicode partial-block characters
// (▏▎▍▌▋▊▉█) for sub-character resolution — 80 distinct fill levels
// in 10 characters. Generated by: roboduck gorun -f hSh-0xPs-genbars.go
var barStrings = [101]string{
	"░░░░░░░░░░", // 0%
	"░░░░░░░░░░", // 1%
	"▏░░░░░░░░░", // 2%
	"▎░░░░░░░░░", // 3%
	"▍░░░░░░░░░", // 4%
	"▌░░░░░░░░░", // 5%
	"▌░░░░░░░░░", // 6%
	"▋░░░░░░░░░", // 7%
	"▊░░░░░░░░░", // 8%
	"▉░░░░░░░░░", // 9%
	"█░░░░░░░░░", // 10%
	"█░░░░░░░░░", // 11%
	"█▏░░░░░░░░", // 12%
	"█▎░░░░░░░░", // 13%
	"█▍░░░░░░░░", // 14%
	"█▌░░░░░░░░", // 15%
	"█▌░░░░░░░░", // 16%
	"█▋░░░░░░░░", // 17%
	"█▊░░░░░░░░", // 18%
	"█▉░░░░░░░░", // 19%
	"██░░░░░░░░", // 20%
	"██░░░░░░░░", // 21%
	"██▏░░░░░░░", // 22%
	"██▎░░░░░░░", // 23%
	"██▍░░░░░░░", // 24%
	"██▌░░░░░░░", // 25%
	"██▌░░░░░░░", // 26%
	"██▋░░░░░░░", // 27%
	"██▊░░░░░░░", // 28%
	"██▉░░░░░░░", // 29%
	"███░░░░░░░", // 30%
	"███░░░░░░░", // 31%
	"███▏░░░░░░", // 32%
	"███▎░░░░░░", // 33%
	"███▍░░░░░░", // 34%
	"███▌░░░░░░", // 35%
	"███▌░░░░░░", // 36%
	"███▋░░░░░░", // 37%
	"███▊░░░░░░", // 38%
	"███▉░░░░░░", // 39%
	"████░░░░░░", // 40%
	"████░░░░░░", // 41%
	"████▏░░░░░", // 42%
	"████▎░░░░░", // 43%
	"████▍░░░░░", // 44%
	"████▌░░░░░", // 45%
	"████▌░░░░░", // 46%
	"████▋░░░░░", // 47%
	"████▊░░░░░", // 48%
	"████▉░░░░░", // 49%
	"█████░░░░░", // 50%
	"█████░░░░░", // 51%
	"█████▏░░░░", // 52%
	"█████▎░░░░", // 53%
	"█████▍░░░░", // 54%
	"█████▌░░░░", // 55%
	"█████▌░░░░", // 56%
	"█████▋░░░░", // 57%
	"█████▊░░░░", // 58%
	"█████▉░░░░", // 59%
	"██████░░░░", // 60%
	"██████░░░░", // 61%
	"██████▏░░░", // 62%
	"██████▎░░░", // 63%
	"██████▍░░░", // 64%
	"██████▌░░░", // 65%
	"██████▌░░░", // 66%
	"██████▋░░░", // 67%
	"██████▊░░░", // 68%
	"██████▉░░░", // 69%
	"███████░░░", // 70%
	"███████░░░", // 71%
	"███████▏░░", // 72%
	"███████▎░░", // 73%
	"███████▍░░", // 74%
	"███████▌░░", // 75%
	"███████▌░░", // 76%
	"███████▋░░", // 77%
	"███████▊░░", // 78%
	"███████▉░░", // 79%
	"████████░░", // 80%
	"████████░░", // 81%
	"████████▏░", // 82%
	"████████▎░", // 83%
	"████████▍░", // 84%
	"████████▌░", // 85%
	"████████▌░", // 86%
	"████████▋░", // 87%
	"████████▊░", // 88%
	"████████▉░", // 89%
	"█████████░", // 90%
	"█████████░", // 91%
	"█████████▏", // 92%
	"█████████▎", // 93%
	"█████████▍", // 94%
	"█████████▌", // 95%
	"█████████▌", // 96%
	"█████████▋", // 97%
	"█████████▊", // 98%
	"█████████▉", // 99%
	"██████████", // 100%
}

// nullBar is the muted bar shown before the first API call.
var nullBar = mutedStyle.Render(barStrings[0] + " –")

// Context bar styles — one per threshold, created once at package level.
var (
	barStyleMint    = lipgloss.NewStyle().Foreground(colorMint)
	barStyleWarning = lipgloss.NewStyle().Foreground(colorWarning)
	barStyleError   = lipgloss.NewStyle().Foreground(colorError)
)

// writeContextBar appends a styled 10-character progress bar with percentage to b.
// Colour shifts by threshold: mint <70%, warning 70–89%, error 90%+.
// When pct is nil (before first API call), writes a muted empty bar.
func writeContextBar(b *strings.Builder, pct *float64) {
	if pct == nil {
		b.WriteString(nullBar)
		return
	}

	p := min(max(int(math.Round(*pct)), 0), 100)

	label := barStrings[p] + " " + strconv.Itoa(p) + "%"

	var style lipgloss.Style
	switch {
	case p >= 90:
		style = barStyleError
	case p >= 70:
		style = barStyleWarning
	default:
		style = barStyleMint
	}

	b.WriteString(style.Render(label))
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

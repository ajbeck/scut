# Status Line

`botctrl claude status-line`

## Overview

Renders a persistent status bar for the Claude Code terminal. Claude Code pipes a JSON session snapshot to stdin after each assistant message (debounced at 300ms); the command prints styled text to stdout.

Design priorities: **zero subprocess overhead** and **sub-millisecond execution**. All data that could be obtained via shell commands (git branch, path resolution, worktree status) is computed in-process using Go libraries.

## Colour Palette

Colours are drawn from the [Data Monocle colour system](https://github.com/ajbeck/data-monocle). Text elements (path, branch) use the **300 stop** — brighter/lighter for readability on dark backgrounds, paired with bold. Accent/status elements use the **400 stop**.

```
Role                  Palette    Stop   Hex        lipgloss Usage
Path / directory      Sky        300    #5CB3FF    lipgloss.Color("#5CB3FF") + Bold
Git branch            Violet     300    #A47DFF    lipgloss.Color("#A47DFF") + Bold
Separators / muted    Slate      400    #6C757D    lipgloss.Color("#6C757D")
Staged / clean ✓      Mint       400    #00D97F    lipgloss.Color("#00D97F")
Unstaged file count   Warning    400    #E9A512    lipgloss.Color("#E9A512")
Ahead arrows          Mint       400    #00D97F    lipgloss.Color("#00D97F")
Behind arrows         Warning    400    #E9A512    lipgloss.Color("#E9A512")
Context bar <70%      Mint       400    #00D97F    lipgloss.Color("#00D97F")
Context bar 70–82%    Warning    400    #E9A512    lipgloss.Color("#E9A512")
Context bar 83%+      Error      400    #F03E3E    lipgloss.Color("#F03E3E")
Context bar unfilled  Slate      400    #6C757D    lipgloss.Color("#6C757D")
Compaction marker     Error      400    #F03E3E    lipgloss.Color("#F03E3E")
```

All colours are specified as true-colour hex values via `lipgloss.Color`. In lipgloss v2, `Render()` always emits true-colour ANSI; downsampling for lower-capability terminals happens at the output layer (`Sprint`/`Fprint`/`Writer`), which we bypass since Claude Code's status line consumer handles ANSI directly.

### Adding colours

When adding new status line segments, pick from the Data Monocle 400 stops:

```
Available    Hex
Coral        #FF6347
Sunshine     #FFD700
Tangerine    #FF9500
Bubblegum    #FF47AE
Lagoon       #00CCB8
```

For status/semantic colours use the status palettes at the 400 stop:

```
Status     Hex
Success    #22DD5E
Warning    #E9A512
Error      #F03E3E
```

## Formatting

### Output format

```
██████████████│ 50% | O4.6 | botctrl/internal/cmd | getting-started ✓ ↑1
└mint─┘└slate┘│      └muted┘ └── sky (bold) ──────┘   └─ violet (bold) ─┘
               └error┘                                  └mint┘ └mint┘
     └─ slate (separators) ─────────────────────────┘
```

- **Context bar** (first segment): 20-character progress bar (19 fill + 1 marker). Filled portion (`█`) in accent colour, half-block transition (`▌`) with FG=accent BG=slate, unfilled portion (`█`) in muted slate. A red `│` marker sits at position 16 (83% of 20), marking the auto-compaction threshold. Colour shifts by threshold. Always shown — displays `████████████████│███ –` in muted slate before the first API call. Uses `used_percentage` from the session snapshot directly.
- **Model**: abbreviated model label in muted slate (e.g., `S4.5`, `O4.6-1M`). Parsed from `model.id` — strips domain prefixes, extracts family initial and version segments. Appends `-1M` when the ID carries the `[1m]` marker (e.g. `claude-opus-4-7[1m]`). The `context_window_size` field is not used: on Bedrock it does not reliably distinguish the 1M variant from the default.
- **Path**: current working directory relative to the git repository root. Bold sky 300. The repo name is the first segment (e.g., `botctrl/internal/cmd`). Long paths are compacted by collapsing intermediate segments to their first character; falls back to `first/…/last` with truncation. Max 25 characters. If not in a git repo, the path is relative to `$HOME` prefixed with `~`.
- **Branch**: the current git branch from HEAD. Bold violet 300. Truncated to 20 characters. Omitted if not in a git repo or HEAD is detached.
- **Git indicators**: `✓` (mint) when clean, otherwise `+N` (staged, mint) and `~N` (unstaged/untracked, warning amber). `↑N` (ahead, mint) and `↓N` (behind, warning) show divergence from `origin/<branch>` based on last-fetch state.
- **Separators**: `|` in muted slate between each segment.

### Styling library

[lipgloss v2](https://github.com/charmbracelet/lipgloss) (`charm.land/lipgloss/v2`) generates ANSI escape codes. In v2, `Style.Render()` always emits full true-colour ANSI — colour downsampling is a separate output concern (via `Sprint`/`Fprint`/`Writer`). Since Claude Code's status line consumer understands ANSI, we write `Render()` output directly without downsampling. This ensures colours survive the stdin/stdout pipe that Claude Code uses to capture the status line. Styles are defined as package-level `lipgloss.Style` values:

```go
pathStyle      = lipgloss.NewStyle().Foreground(colorSky).Bold(true)
branchStyle    = lipgloss.NewStyle().Foreground(colorViolet).Bold(true)
sepStyle       = lipgloss.NewStyle().Foreground(colorSlate)
gitStagedStyle = lipgloss.NewStyle().Foreground(colorMint)
gitDirtyStyle  = lipgloss.NewStyle().Foreground(colorWarning)
```

lipgloss handles:

- True colour → 256 colour → 16 colour downgrading based on terminal capabilities
- Correct ANSI reset sequences
- Width calculation accounting for wide/combining characters

## Performance

### Single repo open

The `gitInfo` struct wraps `*gogit.Repository` and `*gogit.Worktree`, opened once via `openGit(cwd)`. All git queries (`resolve`, `dirtyCount`) use this shared handle. There is no second `PlainOpen` call.

### Concurrent data collection

Four goroutines run in parallel via `sync.WaitGroup.Go`:

```
Goroutine              What it does                                      Why it's separate
gi.resolve(cwd)        Path + branch from HEAD ref                       Fast (ref lookup), but independent
gi.dirtyCount()        Walk worktree status for staged/unstaged counts   Slowest — worktree diff against index
gi.aheadBehind()       Ahead/behind counts vs origin/<branch>            Commit graph walk, independent of status
renderContextBar(pct)  Build styled progress bar string                  Pure computation, no I/O
```

The `wg.Wait()` gate ensures all data is ready before the final string assembly.

### go-git StatusStrategy

`Worktree.Status()` uses the default `Empty` strategy, which starts from an empty map and only populates entries for _changed_ files. This avoids walking the full index (which the `Preload` strategy does). The tradeoff: unmodified files may be misreported as untracked (go-git #119), but we never inspect unmodified files — we only count dirty entries.

## Git Integration

Uses [go-git/go-git](https://github.com/go-git/go-git) in pure Go — no `git` subprocess.

### gitInfo struct

```go
type gitInfo struct {
    repo *gogit.Repository
    wt   *gogit.Worktree
}
```

Created once via `openGit(dir)`. If `dir` is not inside a repo, both fields are nil and all methods return zero values gracefully.

### Operations

```
Method           What                            go-git API
resolve(cwd)     Relative path + branch name     wt.Filesystem.Root(), repo.Head().Name().Short()
dirtyCount()     Staged and unstaged file counts  wt.Status() → iterate FileStatus.Staging / .Worktree
aheadBehind()    Commits ahead/behind origin      repo.Reference() + Commit.MergeBase() + concurrent repo.Log() walks
```

### StatusCode reference

go-git's `StatusCode` is a byte matching git's short format:

```
Code    Constant              Meaning
' '     Unmodified            No changes
'?'     Untracked             New file not in index
'M'     Modified              Content changed
'A'     Added                 New file staged
'D'     Deleted               File removed
'R'     Renamed               File renamed (Extra has old name)
'C'     Copied                File copied
'U'     UpdatedButUnmerged    Merge conflict
```

`dirtyCount` classifies: `Staging != Unmodified && Staging != Untracked` → staged; `Worktree != Unmodified` → unstaged.

### Graceful degradation

If any step fails (not a git repo, bare repo, detached HEAD), the command degrades:

```
Failure               Path fallback    Branch     Dirty indicators
Not a git repo        ~/relative       omitted    omitted
Bare repo (no wt)     ~/relative       omitted    omitted
Detached HEAD         repo-relative    omitted    shown if available
Status error          repo-relative    shown      0, 0
```

## Input

Deserialized from stdin as `claudecode.StatusLineInput`. This is **not** a hook event — it's a periodic session snapshot. The full type is defined in `hooks/claudecode/statusline.go`.

Key fields used by the current implementation:

```
Field                                Type       Usage
workspace.current_dir                string     Current working directory (preferred over cwd)
cwd                                  string     Fallback for workspace.current_dir
context_window.used_percentage       *float64   Context usage percentage — drives bar fill and colour
```

### Context percentage

The bar uses `used_percentage` from the session snapshot directly. This is the cumulative context usage metric that tracks auto-compaction (which triggers at 83%). Note: `total_input_tokens` and `total_output_tokens` are **per-call** values, not cumulative — they are not suitable for computing session-level context usage.

Fields available for future segments:

```
Field                                      Type      Description
model.id, model.display_name              string    Active model
cost.total_cost_usd                       float64   Session cost
cost.total_duration_ms                    int64     Wall-clock time
cost.total_api_duration_ms                int64     Time waiting on API
context_window.remaining_percentage       *float64  Inverse of used_percentage
rate_limits.five_hour.used_percentage     float64   5-hour rate limit (Pro/Max only)
rate_limits.seven_day.used_percentage     float64   7-day rate limit (Pro/Max only)
cost.total_lines_added / removed          int       Code churn metrics
vim.mode                                  string    Vim mode (NORMAL/INSERT)
agent.name                                string    Agent name (with --agent)
worktree.name                             string    Worktree name (with --worktree)
exceeds_200k_tokens                       bool      Whether last response exceeded 200k
```

## Context Bar — Two-Tone Progress Bar

### Design

The context bar is a 20-character progress bar that displays context window usage as a percentage. A red `│` marker sits at character position 16 (83% of 20), splitting the fill area into 16 pre-marker and 3 post-marker characters (19 fillable total). The fill area uses the left half-block character (`▌`) with foreground + background colours for sub-character resolution — **38 distinct fill levels** in 19 characters.

The half-block technique (borrowed from [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) progress bar): `▌` fills the left half of the cell in the accent foreground colour while the right half shows the slate background — matching the solid `█` unfilled blocks.

The bar is rendered by walking each position in the 20-character width. At the marker position, a red `│` is inserted; all other positions are fill characters: accent-coloured `█` for filled, optional `▌` half-block transition (FG=accent, BG=slate), and muted slate `█` for unfilled. The marker provides a fixed visual landmark so you can gauge proximity to auto-compaction at a glance.

### Runtime computation

The bar is computed at render time inside a goroutine that runs in parallel with the git operations. The math:

```go
halves := p * fillArea * 2 / 100  // total fill in halves (0–28)
full   := halves / 2              // fully filled characters
half   := halves % 2              // 0 or 1 (half-block transition)
empty  := fillArea - full
if half > 0 { empty-- }
```

A `barTheme` struct groups the three accent-dependent styles (solid fill, half-block transition, compaction marker background) so the threshold switch selects a single theme rather than three individual styles. Styled block strings (`filledBlock`, `halfBlock`, `emptyBlock`, marker variants) are precomputed once per render before the loop, avoiding repeated `lipgloss.Render()` allocations.

The styled segments are assembled into a string: accent-coloured full blocks, optional half-block with FG+BG, muted empty blocks, and the red compaction marker. Benchmarks show ~4–6µs per render on Apple M4 Pro — negligible against the 300ms debounce interval, and the render runs in parallel with the git operations so it's effectively free.

### Render path

`renderContextBar` returns the complete styled bar string:

1. Clamp percentage to `[0, 100]` using `min(max(...))` builtins.
2. Compute filled/half/empty character counts within the 19-char fill area.
3. Select a `barTheme` by threshold and precompute styled block strings.
4. Walk positions 0–19: at `markerPos` (16) emit the red `│`; otherwise emit the next fill character — accent `█` for filled, `▌` (FG=accent, BG=slate) for the half-block transition, or muted `█` for unfilled.
5. Append the integer percentage label in the accent colour.

When the percentage is nil (before the first API call), a precomputed `nullBar` is returned — the muted empty bar with the red marker and an en-dash instead of a number.

## Code

- **Types**: `hooks/claudecode/statusline.go`
- **Command**: `internal/cmd/claude/statusline.go`

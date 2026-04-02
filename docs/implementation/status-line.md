# Status Line

`botctrl claude status-line`

## Overview

Renders a persistent status bar for the Claude Code terminal. Claude Code pipes a JSON session snapshot to stdin after each assistant message (debounced at 300ms); the command prints styled text to stdout.

Design priorities: **zero subprocess overhead** and **sub-millisecond execution**. All data that could be obtained via shell commands (git branch, path resolution, worktree status) is computed in-process using Go libraries.

## Colour Palette

Colours are drawn from the [Data Monocle colour system](https://github.com/ajbeck/data-monocle), using the **400 stop** of each named palette. The 400 stop is the "primary" shade — vibrant enough for terminal accents on both dark and light backgrounds.

| Role | Palette | Hex | lipgloss Usage |
|------|---------|-----|----------------|
| Path / directory | Sky | `#2196F5` | `lipgloss.Color("#2196F5")` |
| Git branch | Violet | `#8B5CF6` | `lipgloss.Color("#8B5CF6")` |
| Separators / muted | Slate | `#6C757D` | `lipgloss.Color("#6C757D")` |
| Staged file count | Mint | `#00D97F` | `lipgloss.Color("#00D97F")` |
| Unstaged file count | Warning | `#E9A512` | `lipgloss.Color("#E9A512")` |
| Context bar <70% | Mint | `#00D97F` | `lipgloss.Color("#00D97F")` |
| Context bar 70–89% | Warning | `#E9A512` | `lipgloss.Color("#E9A512")` |
| Context bar 90%+ | Error | `#F03E3E` | `lipgloss.Color("#F03E3E")` |

All colours are specified as true-colour hex values via `lipgloss.Color`. Terminals that don't support true colour will get the closest ANSI approximation automatically — lipgloss handles colour profile detection and downgrading.

### Adding colours

When adding new status line segments, pick from the Data Monocle 400 stops:

| Available | Hex |
|-----------|-----|
| Coral | `#FF6347` |
| Sunshine | `#FFD700` |
| Tangerine | `#FF9500` |
| Bubblegum | `#FF47AE` |
| Lagoon | `#00CCB8` |

For status/semantic colours use the status palettes at the 400 stop:

| Status | Hex |
|--------|-----|
| Success | `#22DD5E` |
| Warning | `#E9A512` |
| Error | `#F03E3E` |

## Formatting

### Output format

```
botctrl/internal/cmd | getting-started +2 ~5 | ██░░░░░░░░ 25%
└─ sky ─────────────┘   └─ violet ───┘         └─ mint ──────┘
                           └mint┘└warn┘
         └─ slate (separators) ────────┘
```

- **Path**: current working directory relative to the git repository root. The repo name is the first segment (e.g., `botctrl/internal/cmd`). If not in a git repo, the path is relative to `$HOME` prefixed with `~`.
- **Branch**: the current git branch from HEAD. Omitted if not in a git repo or HEAD is detached.
- **Dirty indicators**: `+N` (staged, mint) and `~N` (unstaged/untracked, warning amber). Shown next to the branch; omitted when clean.
- **Context bar**: 10-character progress bar (`█` filled, `░` empty) with integer percentage. Colour shifts by threshold. Always shown — displays `░░░░░░░░░░ –` in muted slate before the first API call when `used_percentage` is null.
- **Separators**: pipe `|` in muted slate between each segment.

### Styling library

[charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) generates ANSI escape codes. Styles are defined as package-level `lipgloss.Style` values:

```go
pathStyle      = lipgloss.NewStyle().Foreground(colorSky)
branchStyle    = lipgloss.NewStyle().Foreground(colorViolet)
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

Three goroutines run in parallel via `sync.WaitGroup.Go`:

| Goroutine | What it does | Why it's separate |
|-----------|-------------|-------------------|
| `gi.resolve(cwd)` | Path + branch from HEAD ref | Fast (ref lookup), but independent |
| `gi.dirtyCount()` | Walk worktree status for staged/unstaged counts | Slowest — worktree diff against index |
| `renderContextBar(pct)` | Build styled progress bar string | Pure computation, no I/O |

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

| Method | What | go-git API |
|--------|------|------------|
| `resolve(cwd)` | Relative path + branch name | `wt.Filesystem.Root()`, `repo.Head().Name().Short()` |
| `dirtyCount()` | Staged and unstaged file counts | `wt.Status()` → iterate `FileStatus.Staging` / `.Worktree` |

### StatusCode reference

go-git's `StatusCode` is a byte matching git's short format:

| Code | Constant | Meaning |
|------|----------|---------|
| `' '` | `Unmodified` | No changes |
| `'?'` | `Untracked` | New file not in index |
| `'M'` | `Modified` | Content changed |
| `'A'` | `Added` | New file staged |
| `'D'` | `Deleted` | File removed |
| `'R'` | `Renamed` | File renamed (Extra has old name) |
| `'C'` | `Copied` | File copied |
| `'U'` | `UpdatedButUnmerged` | Merge conflict |

`dirtyCount` classifies: `Staging != Unmodified && Staging != Untracked` → staged; `Worktree != Unmodified` → unstaged.

### Graceful degradation

If any step fails (not a git repo, bare repo, detached HEAD), the command degrades:

| Failure | Path fallback | Branch | Dirty indicators |
|---------|---------------|--------|------------------|
| Not a git repo | `~/relative` | omitted | omitted |
| Bare repo (no worktree) | `~/relative` | omitted | omitted |
| Detached HEAD | repo-relative | omitted | shown if available |
| Status error | repo-relative | shown | `0, 0` |

## Input

Deserialized from stdin as `claudecode.StatusLineInput`. This is **not** a hook event — it's a periodic session snapshot. The full type is defined in `hooks/claudecode/statusline.go`.

Key fields used by the current implementation:

| Field | Type | Usage |
|-------|------|-------|
| `workspace.current_dir` | `string` | Current working directory (preferred over `cwd`) |
| `cwd` | `string` | Fallback for `workspace.current_dir` |
| `context_window.used_percentage` | `*float64` | Context bar fill level (null → muted empty bar) |

Fields available for future segments:

| Field | Type | Description |
|-------|------|-------------|
| `model.id`, `model.display_name` | `string` | Active model |
| `cost.total_cost_usd` | `float64` | Session cost |
| `cost.total_duration_ms` | `int64` | Wall-clock time |
| `cost.total_api_duration_ms` | `int64` | Time waiting on API |
| `context_window.context_window_size` | `int` | Max context tokens (200k or 1M) |
| `context_window.remaining_percentage` | `*float64` | Inverse of used_percentage |
| `rate_limits.five_hour.used_percentage` | `float64` | 5-hour rate limit (Pro/Max only) |
| `rate_limits.seven_day.used_percentage` | `float64` | 7-day rate limit (Pro/Max only) |
| `cost.total_lines_added` / `removed` | `int` | Code churn metrics |
| `vim.mode` | `string` | Vim mode (`NORMAL`/`INSERT`) |
| `agent.name` | `string` | Agent name (with `--agent`) |
| `worktree.name` | `string` | Worktree name (with `--worktree`) |
| `exceeds_200k_tokens` | `bool` | Whether last response exceeded 200k |

## Context Bar — High-Resolution Progress Bar

### Design

The context bar is a 10-character progress bar that displays context window usage as a percentage. It uses Unicode left-block partial characters to achieve sub-character resolution — **80 distinct fill levels** in 10 characters, compared to 10 levels with full blocks alone.

The eight partial-block characters, from thinnest to full:

```
▏ ▎ ▍ ▌ ▋ ▊ ▉ █
```

Each character position can display one of 8 fill levels (0/8 through 8/8), giving 10 × 8 = 80 possible fill positions. The empty portion uses `░` (light shade) for visual contrast against unfilled space.

### Lookup table vs runtime math

The bar strings are stored as a hardcoded `[101]string` array indexed by integer percentage (0–100). At render time, displaying a bar is a single array index — no arithmetic, no string building, no allocation.

Three approaches were considered:

| Approach | Render cost | Startup cost | Allocations per call |
|----------|------------|--------------|---------------------|
| Runtime math (`strings.Repeat` + partial block selection) | ~200ns | None | 3+ (string concat) |
| `init()` computed lookup | Array index | Loop + string building | 0 |
| Hardcoded lookup (current) | Array index | None | 0 |

The hardcoded approach wins on both axes: zero allocation at render time and zero startup cost. The values are computed once by a generation script and embedded directly in source.

### Generation script

The lookup table was generated by a Go script run via `roboduck gorun`. The script computes how many "eighths" of fill each percentage needs across 10 character positions, then assembles the corresponding Unicode characters:

```go
blocks := []string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
const w = 10

for pct := range 101 {
    eighths := pct * w * 8 / 100  // total fill in eighths (0–80)
    full := eighths / 8           // fully filled characters
    partial := eighths % 8        // fractional part (0–7)
    empty := w - full
    if partial > 0 {
        empty--
    }
    // Assemble: full █ blocks + partial block + empty ░ blocks
}
```

The generation script source is referenced in the `barStrings` declaration comment. To regenerate: `roboduck gorun -f <script-path>`, then paste the output into the array literal.

### Render path

`renderContextBar` is the only function that reads the lookup table:

1. Clamp percentage to `[0, 100]` using `min(max(...))` builtins.
2. Index into `barStrings[p]` — single array access.
3. Append the integer percentage label via `fmt.Sprintf`.
4. Apply threshold colour (mint <70%, warning 70–89%, error ≥90%) via lipgloss.

When the percentage is nil (before the first API call), a precomputed `nullBar` is returned — the muted empty bar with an en-dash instead of a number.

## Code

- **Types**: `hooks/claudecode/statusline.go`
- **Command**: `internal/cmd/claude/statusline.go`

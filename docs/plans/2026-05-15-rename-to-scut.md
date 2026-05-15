# Rename botctrl → scut — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the project from `botctrl` to `scut` across the Go module path, binary name, GitHub repository, local working directory, log directory, Claude Code memory directory, and every textual reference in code and docs — without changing behavior.

**Architecture:** Two atomic commits on the `getting-started` branch (`feat(claude): add config command…` then `chore: rename botctrl to scut`), followed by a coordinated GitHub-side rename (`gh repo rename`), local filesystem move, and Claude memory directory migration. The diff is large but mechanical.

**Tech Stack:** Go 1.26, Mage build system, kong CLI, afero filesystem, `encoding/json/v2` (build-tagged with `goexperiment.jsonv2`), `git`, GitHub `gh` CLI. macOS BSD `sed` (use `sed -i ''` syntax).

**Spec:** `docs/specs/2026-05-15-rename-to-scut.html` — read first for context.

**Push policy:** Every `git push` and remote-affecting `gh` command requires explicit user confirmation at the time. Tasks 13 and 14 are gated as **STOP & CONFIRM**.

---

## Task 1: Pre-flight verification

**Files:** none modified — read-only checks.

- [ ] **Step 1: Verify branch and uncommitted state**

```bash
git status
git log -1 --oneline
```

Expected: branch is `getting-started`; status shows untracked `internal/cmd/claude/config/` directory, untracked `docs/specs/2026-05-15-github-pages-research.html`, untracked `docs/specs/2026-05-15-rename-to-scut.html`, untracked `docs/config-command.html`, untracked `docs/plans/2026-05-15-rename-to-scut.md`, modified `CLAUDE.md`, modified `README.md`, modified `docs/kong-base-setup.html`, modified `internal/cmd/claude/claude.go`, plus the five pre-existing `mage fmt` artifacts (`internal/cmd/claude/hook/posttooluse.go`, `internal/cmd/format/format.go`, `internal/format/format.go`, `internal/format/format_go_test.go`, `internal/format/format_markdown_test.go`).

If status diverges from the above, STOP and report — do not proceed.

- [ ] **Step 2: Verify tests are green**

```bash
mage test
```

Expected: every package reports `ok`. No `FAIL` lines.

If tests fail, STOP and report.

- [ ] **Step 3: Confirm ready**

No commit yet. State is verified.

---

## Task 2: Commit config-command (Phase 0)

**Files:** stages every uncommitted file currently in the working tree.

- [ ] **Step 1: Stage explicit paths**

```bash
git add \
  internal/cmd/claude/config/ \
  internal/cmd/claude/claude.go \
  internal/cmd/claude/hook/posttooluse.go \
  internal/cmd/format/format.go \
  internal/format/format.go \
  internal/format/format_go_test.go \
  internal/format/format_markdown_test.go \
  CLAUDE.md \
  README.md \
  docs/kong-base-setup.html \
  docs/config-command.html \
  docs/specs/2026-05-15-github-pages-research.html \
  docs/specs/2026-05-15-rename-to-scut.html \
  docs/plans/2026-05-15-rename-to-scut.md
```

- [ ] **Step 2: Verify staged set**

```bash
git status
```

Expected: every line under "Changes to be committed" matches the paths above. No "Untracked files" remaining (other than `bin/`, `.DS_Store`, or other gitignored noise).

- [ ] **Step 3: Commit**

```bash
git commit -m "$(cat <<'EOF'
feat(claude): add config command for managing settings.json

Adds botctrl claude config install/uninstall/status. The command
group manages Claude Code's settings.json on behalf of the user —
install writes (or merges) every hook handler and the status line
into the chosen scope, uninstall reverses, status reports.

Settings are modeled with encoding/json/v2 and an inlined Foreign
fallback map so non-botctrl top-level keys round-trip verbatim.
Output is deterministic via json.Deterministic + a single
marshalSettings helper, making install idempotent.

Also includes docs/config-command.html, the CLAUDE.md index and
commit-time check rule entries, kong-base-setup diagram update,
README rewrites that defer to the new command, the GitHub Pages
research spec, and the rename-to-scut spec and plan.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 4: Verify**

```bash
git log -1 --stat
```

Expected: single commit, subject `feat(claude): add config command for managing settings.json`, files match Step 1.

---

## Task 3: Bulk-replace the full module path (Phase 1 Step 1)

**Files:** every `.go`, `.mod`, `.sum`, `.html`, `.md` containing `github.com/ajbeck/scut`.

- [ ] **Step 1: Survey current scope**

```bash
grep -rln 'github.com/ajbeck/scut' --include='*.go' --include='*.mod' --include='*.sum' --include='*.html' --include='*.md' .
```

Expected: ~13 Go files + go.mod + a few docs. Note the count.

- [ ] **Step 2: Apply the replacement**

```bash
grep -rl 'github.com/ajbeck/scut' --include='*.go' --include='*.mod' --include='*.sum' --include='*.html' --include='*.md' . \
  | xargs sed -i '' 's|github.com/ajbeck/scut|github.com/ajbeck/scut|g'
```

Note: on macOS BSD `sed`, the `-i ''` syntax (empty backup suffix) is required.

- [ ] **Step 3: Verify the replacement is complete**

```bash
grep -rn 'github.com/ajbeck/scut' --include='*.go' --include='*.mod' --include='*.sum' --include='*.html' --include='*.md' .
```

Expected: no matches.

- [ ] **Step 4: Verify go.mod**

```bash
head -1 go.mod
```

Expected: `module github.com/ajbeck/scut`

- [ ] **Step 5: Compile check**

```bash
mage vet
```

Expected: clean. (Note: `cmd/botctrl/` directory still exists with its old name — that's fine; Go cares about module path in go.mod, not directory names.)

---

## Task 4: Rename `cmd/botctrl/` directory (Phase 1 Step 2)

**Files:** `cmd/botctrl/` → `cmd/scut/`.

- [ ] **Step 1: Move the directory with git tracking**

```bash
git mv cmd/botctrl cmd/scut
```

- [ ] **Step 2: Verify**

```bash
ls cmd/
```

Expected: directory `scut/` exists; no `botctrl/`.

- [ ] **Step 3: Compile check**

```bash
mage build
```

Expected: builds successfully. Binary still named `bin/botctrl` because `magefiles/helpers.go` still says `binaryName = "botctrl"` — that gets fixed in Task 9. Don't try to run the binary yet.

---

## Task 5: Replace `owns()` prefix strings (Phase 1 Step 3a)

**Files:**
- Modify: `internal/cmd/claude/config/ownership.go`
- Modify: `internal/cmd/claude/config/ownership_test.go`

- [ ] **Step 1: Update ownership.go**

In `internal/cmd/claude/config/ownership.go`, three replacements within the `owns` function:

Replace `// owns reports whether command is a botctrl invocation we should manage.` with `// owns reports whether command is a scut invocation we should manage.`

Replace `// First scope: exact "botctrl" or "botctrl " / "botctrl\t" prefix on the leading token.` with `// First scope: exact "scut" or "scut " / "scut\t" prefix on the leading token.`

Replace `if c == "botctrl" {` with `if c == "scut" {`

Replace `return strings.HasPrefix(c, "botctrl ") || strings.HasPrefix(c, "botctrl\t")` with `return strings.HasPrefix(c, "scut ") || strings.HasPrefix(c, "scut\t")`

- [ ] **Step 2: Update ownership_test.go**

In `internal/cmd/claude/config/ownership_test.go`, every literal `botctrl` in test fixtures becomes `scut`:

Replace `{"botctrl", true},` with `{"scut", true},`

Replace `{"botctrl claude hook post-tool-use", true},` with `{"scut claude hook post-tool-use", true},`

Replace `{"botctrl\tclaude hook post-tool-use", true},` with `{"scut\tclaude hook post-tool-use", true},`

Replace `// Token boundary: must not match "botctrlsomething".` with `// Token boundary: must not match "scutsomething".`

Replace `{"botctrlsomething", false},` with `{"scutsomething", false},`

Replace `{"botctrlx claude hook post-tool-use", false},` with `{"scutx claude hook post-tool-use", false},`

- [ ] **Step 3: Run the ownership tests**

```bash
GOEXPERIMENT=jsonv2 go test -run TestOwns -count=1 ./internal/cmd/claude/config/
```

Expected: PASS. (Using `go test` directly here is intentional — we are running a single named test for fast feedback. Full `mage test` runs at end of Phase 1.)

---

## Task 6: Replace `install.go` command-string builders and comments (Phase 1 Step 3b)

**Files:**
- Modify: `internal/cmd/claude/config/install.go`
- Modify: `internal/cmd/claude/config/errors.go`
- Modify: `internal/cmd/claude/config/registry.go`
- Modify: `internal/cmd/claude/config/config.go`
- Modify: `internal/cmd/claude/config/status.go`
- Modify: `internal/cmd/claude/config/uninstall.go`

- [ ] **Step 1: Update install.go command-string builders**

In `internal/cmd/claude/config/install.go`:

Replace `// installCmd is the kong leaf for "botctrl claude config install".` with `// installCmd is the kong leaf for "scut claude config install".`

Replace `Command: "botctrl claude " + logPrefix + "status-line",` with `Command: "scut claude " + logPrefix + "status-line",`

Replace `cmd := "botctrl claude " + logPrefix + "hook " + spec.Slug` with `cmd := "scut claude " + logPrefix + "hook " + spec.Slug`

- [ ] **Step 2: Update install.go logPrefix doc-comment**

In `internal/cmd/claude/config/install.go`, the `logPrefix` function's doc comment references `"botctrl claude "`:

Replace `// logPrefix returns the command-string fragment to insert between "botctrl claude "` with `// logPrefix returns the command-string fragment to insert between "scut claude "`

Replace `// Neither flag:     "" (empty — bare "botctrl claude hook <slug>")` with `// Neither flag:     "" (empty — bare "scut claude hook <slug>")`

- [ ] **Step 3: Update package and Cmd doc-comments**

In `internal/cmd/claude/config/config.go`:

Replace `// Package config implements the "botctrl claude config" command group.` with `// Package config implements the "scut claude config" command group.`

Replace `// Cmd is the Kong command group for "botctrl claude config".` with `// Cmd is the Kong command group for "scut claude config".`

In `internal/cmd/claude/config/registry.go`:

Replace `// Package config implements the "botctrl claude config" command group.` with `// Package config implements the "scut claude config" command group.`

Replace `// Slug is the --only token AND the leaf command name under "botctrl claude hook".` with `// Slug is the --only token AND the leaf command name under "scut claude hook".`

In `internal/cmd/claude/config/status.go`:

Replace `// statusCmd is the kong leaf for "botctrl claude config status".` with `// statusCmd is the kong leaf for "scut claude config status".`

In `internal/cmd/claude/config/uninstall.go`:

Replace `// uninstallCmd is the kong leaf for "botctrl claude config uninstall".` with `// uninstallCmd is the kong leaf for "scut claude config uninstall".`

- [ ] **Step 4: Update errors.go**

In `internal/cmd/claude/config/errors.go`:

Replace `// has a statusLine entry whose command does not start with "botctrl ".` with `// has a statusLine entry whose command does not start with "scut ".`

- [ ] **Step 5: Verify package compiles**

```bash
GOEXPERIMENT=jsonv2 go build ./internal/cmd/claude/config/
```

Expected: clean exit, no output.

---

## Task 7: Replace test fixture strings (Phase 1 Step 3c)

**Files:**
- Modify: `internal/cmd/claude/config/settings_test.go`
- Modify: `internal/cmd/claude/config/install_test.go`
- Modify: `internal/cmd/claude/config/uninstall_test.go`
- Modify: `internal/cmd/claude/config/status_test.go`
- Modify: `internal/cmd/claude/config/config_test.go`

- [ ] **Step 1: Update settings_test.go**

In `internal/cmd/claude/config/settings_test.go`:

Replace `StatusLine: &StatusLine{Type: "command", Command: "botctrl claude status-line"},` with `StatusLine: &StatusLine{Type: "command", Command: "scut claude status-line"},`

Replace `Type: "command", Command: "botctrl claude hook post-tool-use", StatusMessage: "Formatting..."` with `Type: "command", Command: "scut claude hook post-tool-use", StatusMessage: "Formatting..."` (preserve surrounding context in the test).

Replace `"statusLine": {"type": "command", "command": "botctrl claude status-line"},` with `"statusLine": {"type": "command", "command": "scut claude status-line"},`

Survey-and-replace any other `botctrl` literals in the file:

```bash
grep -n 'botctrl' internal/cmd/claude/config/settings_test.go
```

For each remaining match, replace `botctrl` with `scut`.

- [ ] **Step 2: Update install_test.go**

In `internal/cmd/claude/config/install_test.go`:

Replace `if !bytes.Contains(data, []byte("botctrl claude --log hook session-start")) {` with `if !bytes.Contains(data, []byte("scut claude --log hook session-start")) {`

Replace `bytes.Contains(data, []byte("botctrl claude --log-level=debug hook session-start"))` with `bytes.Contains(data, []byte("scut claude --log-level=debug hook session-start"))`

Survey remaining matches:

```bash
grep -n 'botctrl' internal/cmd/claude/config/install_test.go
```

For each match, replace `botctrl` with `scut` (CLI invocation strings, expected outputs, comments).

- [ ] **Step 3: Update uninstall_test.go**

In `internal/cmd/claude/config/uninstall_test.go`:

Survey:

```bash
grep -n 'botctrl' internal/cmd/claude/config/uninstall_test.go
```

Replace every match by token substitution (`botctrl` → `scut`). Key sites already identified:

- The JSON fixture containing `"statusLine":{"type":"command","command":"botctrl claude status-line"}` becomes `"statusLine":{"type":"command","command":"scut claude status-line"}`
- The hook fixture `"command":"botctrl claude hook post-tool-use"` becomes `"command":"scut claude hook post-tool-use"`
- The assertion `bytes.Contains(data, []byte("botctrl claude hook post-tool-use"))` becomes `bytes.Contains(data, []byte("scut claude hook post-tool-use"))`
- The error message `t.Errorf("botctrl hook still present after uninstall\n%s", data)` becomes `t.Errorf("scut hook still present after uninstall\n%s", data)`

- [ ] **Step 4: Update status_test.go**

In `internal/cmd/claude/config/status_test.go`:

Replace `if !bytes.Contains(stdout.Bytes(), []byte("botctrl claude status-line")) {` with `if !bytes.Contains(stdout.Bytes(), []byte("scut claude status-line")) {`

Replace `bytes.Contains(stdout.Bytes(), []byte("botctrl claude hook post-tool-use"))` with `bytes.Contains(stdout.Bytes(), []byte("scut claude hook post-tool-use"))`

Survey for any remaining matches and replace.

- [ ] **Step 5: Update config_test.go**

In `internal/cmd/claude/config/config_test.go`:

Replace every `kong.Name("botctrl")` with `kong.Name("scut")`. (There are 3-4 such call sites for various Kong parser fixtures.)

Survey:

```bash
grep -n 'botctrl' internal/cmd/claude/config/config_test.go
```

For each remaining match, replace `botctrl` with `scut`.

- [ ] **Step 6: Run the full config package tests**

```bash
GOEXPERIMENT=jsonv2 go test -count=1 ./internal/cmd/claude/config/
```

Expected: PASS for every test. If a test fails because of a literal string mismatch, the failure message will show the expected vs got string — fix the missed literal and re-run.

---

## Task 8: Replace logging directory path (Phase 1 Step 3d)

**Files:**
- Modify: `internal/logging/logging.go`

- [ ] **Step 1: Update the dirName constant**

In `internal/logging/logging.go`:

Replace `dirName     = ".botctrl/logging"` with `dirName     = ".scut/logging"`

- [ ] **Step 2: Update doc comments referencing the path**

In `internal/logging/logging.go`:

Replace `// Package logging provides structured JSONL logging for botctrl commands.` with `// Package logging provides structured JSONL logging for scut commands.`

Replace `// Log files are written to ~/.botctrl/logging/ with date and component` with `// Log files are written to ~/.scut/logging/ with date and component`

Replace `// ~/.botctrl/logging/YYYYMMDD_<name>.jsonl.` with `// ~/.scut/logging/YYYYMMDD_<name>.jsonl.`

Replace `// logDir returns the absolute path to ~/.botctrl/logging/, creating it` with `// logDir returns the absolute path to ~/.scut/logging/, creating it`

Replace `// to ~/.botctrl/logging/YYYYMMDD_parse-errors.jsonl. It is unconditional —` with `// to ~/.scut/logging/YYYYMMDD_parse-errors.jsonl. It is unconditional —`

- [ ] **Step 3: Verify the logging package**

```bash
GOEXPERIMENT=jsonv2 go test -count=1 ./internal/logging/
```

Expected: PASS.

---

## Task 9: Replace Magefile binary references (Phase 1 Step 3e)

**Files:**
- Modify: `magefiles/helpers.go`
- Modify: `magefiles/build.go`
- Modify: `magefiles/docs.go` (prose only — asset filenames preserved)

- [ ] **Step 1: Update helpers.go**

In `magefiles/helpers.go`:

Replace `binaryName = "botctrl"` with `binaryName = "scut"`

Replace `mainPkg    = "./cmd/botctrl"` with `mainPkg    = "./cmd/scut"`

Replace `versionPkg = "github.com/ajbeck/scut/internal/version"` with `versionPkg = "github.com/ajbeck/scut/internal/version"`

- [ ] **Step 2: Update build.go comments**

In `magefiles/build.go`:

Replace `// Build targets for the botctrl CLI.` with `// Build targets for the scut CLI.`

Replace `// Build compiles the botctrl binary into bin/.` with `// Build compiles the scut binary into bin/.`

- [ ] **Step 3: Update docs.go prose (but NOT the asset filenames)**

In `magefiles/docs.go`, replace `botctrl` with `scut` **only** in prose strings — explicitly preserve the asset filename constants `botctrl-docs.css` and `botctrl-docs.js`.

Specific replacements:

Replace `// docs/botctrl-docs.css and docs/botctrl-docs.js into docs/design-system.html.` — leave the asset filenames as-is but the comment is fine as it's filenames being referenced.

Actually, the asset filenames `botctrl-docs.css` and `botctrl-docs.js` are preserved everywhere per spec §03. Do NOT change lines that contain `botctrl-docs.css` or `botctrl-docs.js` (they're internal asset filenames, not project name references).

Replace the title strings:

`<title>Docs Design System — botctrl</title>` becomes `<title>Docs Design System — scut</title>`
`<title>Docs Design System (Standalone) — botctrl</title>` becomes `<title>Docs Design System (Standalone) — scut</title>`

Replace prose mentions in HTML template strings:

`Copy one existing page (e.g. <code>kong-base-setup.html</code> from the full botctrl docs folder)` becomes `Copy one existing page (e.g. <code>kong-base-setup.html</code> from the full scut docs folder)`

`<p>The botctrl docs design system` (appears twice) becomes `<p>The scut docs design system`

`see <code>docs/design-system.html</code> in the botctrl repository.` becomes `see <code>docs/design-system.html</code> in the scut repository.`

The `<span class="blink">botctrl</span>` footers (appearing twice) become `<span class="blink">scut</span>`.

Survey:

```bash
grep -n 'botctrl' magefiles/docs.go
```

Expected remaining matches after replacement: only `botctrl-docs.css` and `botctrl-docs.js` filename references (which we preserve).

- [ ] **Step 4: Verify build produces bin/scut**

```bash
rm -f bin/botctrl bin/scut
mage build
ls bin/
```

Expected: `bin/scut` exists, no `bin/botctrl`.

---

## Task 10: Replace prose and CLI examples in docs (Phase 1 Step 3f)

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `docs/claude-hook-commands.html`
- Modify: `docs/config-command.html`
- Modify: `docs/design-system.html`
- Modify: `docs/design-system-standalone.html`
- Modify: `docs/kong-base-setup.html`
- Modify: `docs/logging.html`
- Modify: `docs/post-tool-use.html`
- Modify: `docs/status-line.html`
- Modify: `docs/specs/2026-05-14-config-command.html`
- Modify: `docs/specs/2026-05-15-github-pages-research.html`
- **Do NOT modify:** `docs/specs/2026-05-15-rename-to-scut.html` (intentionally documents the rename)

- [ ] **Step 1: Update README.md**

Survey:

```bash
grep -n 'botctrl' README.md
```

For each match, replace `botctrl` with `scut`. Key sites: the `# botctrl` H1 (becomes `# scut`), the `go install github.com/ajbeck/scut@latest` line (already updated to `scut` by Task 3 module path replace), every `botctrl claude config install` CLI example, every `botctrl claude --log hook post-tool-use` example, every `botctrl claude status-line` example, the `botctrl logging clean` examples.

- [ ] **Step 2: Update CLAUDE.md**

Survey:

```bash
grep -n 'botctrl' CLAUDE.md
```

For each match, replace `botctrl` with `scut`. Key sites: the `# botctrl` H1, the `**Module**: github.com/ajbeck/scut` line (already updated by Task 3), every CLI example in the doc, every prose mention of "botctrl" as the project name.

- [ ] **Step 3: Bulk-replace prose in non-spec docs**

For each top-level doc, run a sed token replacement. Asset filenames `botctrl-docs.css` and `botctrl-docs.js` must be preserved — handle those separately.

First, surface the file list:

```bash
for f in docs/claude-hook-commands.html docs/config-command.html docs/design-system.html docs/design-system-standalone.html docs/kong-base-setup.html docs/logging.html docs/post-tool-use.html docs/status-line.html docs/specs/2026-05-14-config-command.html docs/specs/2026-05-15-github-pages-research.html; do
  echo "=== $f ==="
  grep -n 'botctrl' "$f"
done
```

Then apply a sed that preserves the asset filenames by replacing the asset names with a sentinel first, doing the bulk replace, then restoring:

```bash
for f in docs/claude-hook-commands.html docs/config-command.html docs/design-system.html docs/design-system-standalone.html docs/kong-base-setup.html docs/logging.html docs/post-tool-use.html docs/status-line.html docs/specs/2026-05-14-config-command.html docs/specs/2026-05-15-github-pages-research.html; do
  sed -i '' '
    s|botctrl-docs\.css|__ASSET_CSS__|g
    s|botctrl-docs\.js|__ASSET_JS__|g
    s|botctrl|scut|g
    s|__ASSET_CSS__|botctrl-docs.css|g
    s|__ASSET_JS__|botctrl-docs.js|g
  ' "$f"
done
```

- [ ] **Step 4: Special handling for the GitHub Pages research spec**

In `docs/specs/2026-05-15-github-pages-research.html`, the Task 3 module-path replace did not touch `ajbeck.github.io/botctrl/`. The Step 3 sed in this task did replace `botctrl` → `scut` in that URL. Verify:

```bash
grep -n 'github.io' docs/specs/2026-05-15-github-pages-research.html
```

Expected: all URLs now read `ajbeck.github.io/scut/`.

- [ ] **Step 5: Verify the rename spec was NOT touched**

```bash
grep -c 'botctrl' docs/specs/2026-05-15-rename-to-scut.html
```

Expected: a non-zero count. The rename spec intentionally references both names. If the count is zero, something replaced too aggressively — investigate.

- [ ] **Step 6: Final survey across all docs**

```bash
grep -rn 'botctrl' docs/ --include='*.html' --include='*.md' \
  | grep -v 'docs/specs/2026-05-15-rename-to-scut.html' \
  | grep -v 'docs/plans/2026-05-15-rename-to-scut.md' \
  | grep -v 'botctrl-docs'
```

Expected: no matches.

---

## Task 11: Phase 1 final verification (Phase 1 Step 4)

**Files:** none modified — verification only.

- [ ] **Step 1: Format**

```bash
mage fmt
git diff --stat
```

Expected: empty diff. If anything changed, that's prior work that needed reformatting — accept it but note for the commit.

- [ ] **Step 2: Vet**

```bash
mage vet
```

Expected: clean.

- [ ] **Step 3: Test**

```bash
mage test
```

Expected: every package reports `ok`. No `FAIL`.

- [ ] **Step 4: Build**

```bash
mage build
ls bin/
```

Expected: `bin/scut` exists.

- [ ] **Step 5: Full repository audit**

```bash
grep -rn 'botctrl' . \
  --exclude-dir=.git \
  --exclude-dir=bin \
  --exclude='*.sum'
```

Expected matches (preserve these):
- `docs/botctrl-docs.css` and `docs/botctrl-docs.js` references (asset filenames)
- `docs/specs/2026-05-15-rename-to-scut.html` (intentional)
- `docs/plans/2026-05-15-rename-to-scut.md` (intentional — this file)
- `magefiles/docs.go` lines that reference the asset filenames

Unexpected matches anywhere else: STOP and investigate.

---

## Task 12: Commit the rename (Phase 2)

**Files:** stages every modified and renamed file from Tasks 3–10.

- [ ] **Step 1: Stage everything**

```bash
git add -A
git status
```

Inspect output. Expected: many files modified, `cmd/botctrl/` files shown as renames to `cmd/scut/`, no untracked surprises.

- [ ] **Step 2: Commit**

```bash
git commit -m "$(cat <<'EOF'
chore: rename botctrl to scut

Renames the project from botctrl to scut. Touches the Go module
path, the cmd/ directory, the binary name, the owns() prefix
check in the config package, the install registry's emitted
command strings, the log directory path, magefile targets, and
every doc and spec page outside the rename spec/plan themselves.

No behavior change. The diff is mechanical.

The new name is a Bobiverse reference — SCUT is the Subspace
Communications Universal Transceiver, the FTL-comms layer that
links every Bob — that also reads as everyday English (scut work)
and as a Unix tool name.

GitHub repo rename (ajbeck/botctrl -> ajbeck/scut) and the local
directory rename are coordinated separately after this commit
lands.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 3: Verify two-commit history**

```bash
git log -2 --oneline
```

Expected:
```
<new sha>  chore: rename botctrl to scut
<earlier> feat(claude): add config command for managing settings.json
```

---

## Task 13: Push to GitHub (Phase 3 Step 1) — STOP & CONFIRM

**Files:** none modified — push operation only.

- [ ] **Step 1: Show the push diff to the user**

```bash
git log origin/getting-started..HEAD --stat 2>/dev/null || git log -2 --stat
```

Display this output to the user for confirmation.

- [ ] **Step 2: STOP & CONFIRM**

Halt. Ask the user explicitly: "Ready to push the two commits to `origin/getting-started` under the still-`botctrl` GitHub repo name? This is the irreversible-ish point."

Do not proceed without explicit user confirmation.

- [ ] **Step 3: Push (only after confirmation)**

```bash
git push origin getting-started
```

Expected: push succeeds. (The repo at this point is still named `ajbeck/botctrl` on GitHub — that's fine; Git does not validate that the module path in the code matches the remote URL.)

- [ ] **Step 4: Verify**

```bash
git log origin/getting-started -1 --oneline
```

Expected: matches local HEAD.

---

## Task 14: Rename GitHub repo + update local remote (Phase 3 Steps 2–3) — STOP & CONFIRM

**Files:** none modified locally — GitHub-side change + local git config.

- [ ] **Step 1: STOP & CONFIRM**

Halt. Ask the user explicitly: "Ready to rename the GitHub repo `ajbeck/botctrl` → `ajbeck/scut`? This is a public repo rename. GitHub will maintain a permanent HTTP redirect from the old URL."

Do not proceed without explicit user confirmation.

- [ ] **Step 2: Rename the GitHub repo (only after confirmation)**

```bash
gh repo rename scut --repo ajbeck/botctrl --yes
```

Expected output: `✓ Renamed repository ajbeck/botctrl to ajbeck/scut`.

- [ ] **Step 3: Update the local remote URL**

```bash
git remote set-url origin git@github.com:ajbeck/scut.git
git remote -v
```

Expected: both `(fetch)` and `(push)` lines point to `git@github.com:ajbeck/scut.git`. (If the user uses HTTPS remotes instead of SSH, substitute `https://github.com/ajbeck/scut.git`.)

- [ ] **Step 4: Verify connectivity**

```bash
git fetch origin
```

Expected: succeeds silently. No errors about repo not found.

- [ ] **Step 5: Verify GitHub side**

```bash
gh repo view ajbeck/scut --json name,url
```

Expected: JSON output showing `name: scut` and the new URL.

---

## Task 15: Backup Claude memory + rename local working directory (Phase 3 Steps 4–5)

**Files:** filesystem moves outside the repo.

- [ ] **Step 1: Back up the Claude Code memory directory**

```bash
cp -r /Users/aj/.claude/projects/-Users-aj-Developer-repos-ajbeck-botctrl \
      /tmp/claude-memory-botctrl-backup-$(date +%Y%m%d-%H%M)
ls /tmp/claude-memory-botctrl-backup-*
```

Expected: backup directory exists in `/tmp/` containing `memory/MEMORY.md` and the feedback memory files.

- [ ] **Step 2: Rename the local working directory**

```bash
mv ~/Developer/repos/ajbeck/botctrl ~/Developer/repos/ajbeck/scut
ls ~/Developer/repos/ajbeck/
```

Expected: `scut/` exists; no `botctrl/`.

- [ ] **Step 3: SESSION HANDOFF**

⚠️ **STOP — Session boundary.** This Claude Code session's tool calls have a cwd that no longer exists. All further work happens in a fresh Claude Code session opened at the new path.

Instruct the user:
1. Close the current Claude Code session (or leave it idle).
2. Open a fresh terminal.
3. `cd ~/Developer/repos/ajbeck/scut`
4. Start a new Claude Code session there.
5. Resume the plan at Task 16.

The backup at `/tmp/claude-memory-botctrl-backup-<timestamp>/` is the recovery path if anything goes wrong with the memory migration in Task 16.

---

## Task 16: [FRESH SESSION] Migrate Claude memory directory (Phase 3 Step 6)

**Prerequisites:** Fresh Claude Code session started at `~/Developer/repos/ajbeck/scut`.

**Files:** filesystem move of the Claude Code memory directory.

- [ ] **Step 1: Verify session location**

```bash
pwd
```

Expected: `/Users/aj/Developer/repos/ajbeck/scut`.

If the cwd is wrong, STOP and have the user `cd` to the right location and start over.

- [ ] **Step 2: Verify the old memory dir still exists**

```bash
ls /Users/aj/.claude/projects/-Users-aj-Developer-repos-ajbeck-botctrl/memory/ 2>&1
```

Expected: lists `MEMORY.md` plus the feedback memory files (`feedback_never_push_without_asking.md`, `feedback_verification_via_unit_tests.md`).

If the directory is missing, restore from the `/tmp/` backup before continuing.

- [ ] **Step 3: Verify the new memory dir name does not yet exist**

```bash
ls /Users/aj/.claude/projects/-Users-aj-Developer-repos-ajbeck-scut 2>&1
```

Expected: `No such file or directory`. (If Claude Code's new session already auto-created an empty memory dir at this path, that's a problem — the `mv` will fail or merge oddly. If this directory exists, inspect it: if it contains only an auto-generated empty `memory/` subdir, remove it before proceeding; if it has content, STOP and reconcile manually.)

- [ ] **Step 4: Move the memory directory**

```bash
mv /Users/aj/.claude/projects/-Users-aj-Developer-repos-ajbeck-botctrl \
   /Users/aj/.claude/projects/-Users-aj-Developer-repos-ajbeck-scut
```

- [ ] **Step 5: Verify**

```bash
ls /Users/aj/.claude/projects/-Users-aj-Developer-repos-ajbeck-scut/memory/
```

Expected: `MEMORY.md` and the two feedback memory files.

---

## Task 17: [FRESH SESSION] Final verification (Phase 3 Steps 7–8)

**Prerequisites:** Task 16 complete.

**Files:** none modified — verification only.

- [ ] **Step 1: Verify git state**

```bash
git status
git log -3 --oneline
git remote -v
```

Expected:
- `git status` clean (no uncommitted changes).
- `git log` shows the two new commits (rename + feat) on `getting-started`.
- `git remote -v` points to `git@github.com:ajbeck/scut.git`.

- [ ] **Step 2: Verify the build**

```bash
mage fmt
mage vet
mage test
mage build
ls bin/scut
```

Expected: every target clean; `bin/scut` exists.

- [ ] **Step 3: Verify Claude Code memory survived**

In this Claude Code session, ask Claude to recall what's in MEMORY.md (or check directly):

```bash
cat /Users/aj/.claude/projects/-Users-aj-Developer-repos-ajbeck-scut/memory/MEMORY.md
```

Expected: the two memory-index lines:
- `[Never push without asking](feedback_never_push_without_asking.md) — …`
- `[Verification = unit tests, not binary runs](feedback_verification_via_unit_tests.md) — …`

If memory is missing, restore from `/tmp/claude-memory-botctrl-backup-*/` to the new path.

- [ ] **Step 4: Cleanup**

Once memory survival is confirmed, delete the `/tmp/` backup at leisure:

```bash
rm -rf /tmp/claude-memory-botctrl-backup-*
```

Not urgent — leave the backup until the next planning session if uncertain.

---

## Done

Two commits on `getting-started` with the rename complete. GitHub repo renamed. Local directory and Claude memory migrated. Working tree clean, tests green, build produces `bin/scut`.

**Next steps (out of scope for this plan):**
- Merge `getting-started` to `main` (separate decision).
- GitHub Pages workflow + custom domain selection (picks up where we left off after the rename).
- Optional cleanup commit renaming `botctrl-docs.css` / `botctrl-docs.js` to `scut-docs.css` / `scut-docs.js`.

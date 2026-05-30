//go:build goexperiment.jsonv2

// Package doctor implements the "scut doctor" diagnostics command.
package doctor

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	json "encoding/json/v2"

	"github.com/spf13/afero"
)

// Cmd is the Kong command for "scut doctor".
type Cmd struct {
	Claude bool   `help:"Check Claude Code setup."`
	Codex  bool   `help:"Check Codex setup."`
	All    bool   `help:"Check all supported agents."`
	Scope  string `help:"Configuration scope to inspect." default:"both" enum:"project,user,both"`
	JSON   bool   `help:"Emit a structured JSON object instead of human-readable output." name:"json"`
}

type severity string

const (
	severityOK    severity = "ok"
	severityInfo  severity = "info"
	severityWarn  severity = "warn"
	severityError severity = "error"
)

type finding struct {
	Severity severity `json:"severity"`
	Agent    string   `json:"agent,omitzero"`
	Scope    string   `json:"scope,omitzero"`
	Check    string   `json:"check"`
	Path     string   `json:"path,omitzero"`
	Message  string   `json:"message"`
}

type output struct {
	Findings []finding `json:"findings"`
}

type agent string

const (
	agentClaude agent = "claude"
	agentCodex  agent = "codex"
)

var lookPath = exec.LookPath

// Run executes read-only setup diagnostics.
func (c *Cmd) Run(stdout io.Writer, fs afero.Fs, logger *slog.Logger) error {
	_ = logger
	findings, err := c.inspect(fs)
	if err != nil {
		return err
	}
	if c.JSON {
		if err := writeJSON(stdout, output{Findings: findings}); err != nil {
			return err
		}
	} else if err := writeHuman(stdout, findings); err != nil {
		return err
	}
	if hasError(findings) {
		return fmt.Errorf("doctor found errors")
	}
	return nil
}

func (c *Cmd) inspect(fs afero.Fs) ([]finding, error) {
	var findings []finding
	if _, err := lookPath("scut"); err != nil {
		findings = append(findings, finding{
			Severity: severityError,
			Check:    "scut-path",
			Message:  "scut is not discoverable on PATH; generated hook commands use bare scut",
		})
	} else {
		findings = append(findings, finding{
			Severity: severityOK,
			Check:    "scut-path",
			Message:  "scut is discoverable on PATH",
		})
	}

	scopes, err := resolveScopes(c.Scope)
	if err != nil {
		return nil, err
	}
	for _, a := range c.resolveAgents() {
		for _, scope := range scopes {
			switch a {
			case agentClaude:
				findings = append(findings, inspectClaude(fs, scope)...)
			case agentCodex:
				findings = append(findings, inspectCodex(fs, scope)...)
			}
		}
	}
	return findings, nil
}

func (c *Cmd) resolveAgents() []agent {
	if c.All || (!c.Claude && !c.Codex) {
		return []agent{agentClaude, agentCodex}
	}
	var agents []agent
	if c.Claude {
		agents = append(agents, agentClaude)
	}
	if c.Codex {
		agents = append(agents, agentCodex)
	}
	return agents
}

func resolveScopes(scope string) ([]string, error) {
	switch scope {
	case "project":
		return []string{"project"}, nil
	case "user":
		return []string{"user"}, nil
	case "both":
		return []string{"project", "user"}, nil
	default:
		return nil, fmt.Errorf("unknown scope %q", scope)
	}
}

func inspectClaude(fs afero.Fs, scope string) []finding {
	path, err := claudeSettingsPath(scope)
	if err != nil {
		return []finding{errorFinding("claude", scope, "scope", "", err.Error())}
	}
	if !exists(fs, path) {
		return []finding{{
			Severity: severityInfo,
			Agent:    "claude",
			Scope:    scope,
			Check:    "config-exists",
			Path:     path,
			Message:  "Claude settings file not found",
		}}
	}

	var settings claudeSettings
	if err := readJSON(fs, path, &settings); err != nil {
		return []finding{errorFinding("claude", scope, "config-parse", path, err.Error())}
	}

	findings := []finding{{
		Severity: severityOK,
		Agent:    "claude",
		Scope:    scope,
		Check:    "config-parse",
		Path:     path,
		Message:  "Claude settings file parsed",
	}}

	hooks, statusLines := claudeScutEntries(settings)
	entries := append(slices.Clone(hooks), statusLines...)
	if len(entries) == 0 {
		findings = append(findings, finding{
			Severity: severityWarn,
			Agent:    "claude",
			Scope:    scope,
			Check:    "scut-hooks",
			Path:     path,
			Message:  "no scut Claude hooks or status line found",
		})
	} else {
		findings = append(findings, finding{
			Severity: severityOK,
			Agent:    "claude",
			Scope:    scope,
			Check:    "scut-hooks",
			Path:     path,
			Message:  fmt.Sprintf("%d scut Claude hooks and %d status line found", len(hooks), len(statusLines)),
		})
	}
	return append(findings, inspectCommands(fs, "claude", scope, path, entries)...)
}

func inspectCodex(fs afero.Fs, scope string) []finding {
	hooksPath, err := codexHooksPath(scope)
	if err != nil {
		return []finding{errorFinding("codex", scope, "scope", "", err.Error())}
	}
	configPath, err := codexConfigPath(scope)
	if err != nil {
		return []finding{errorFinding("codex", scope, "scope", "", err.Error())}
	}

	var findings []finding
	configInfo := inspectCodexConfig(fs, scope, configPath, exists(fs, hooksPath))
	findings = append(findings, configInfo...)

	if scope == "project" && exists(fs, filepath.Dir(hooksPath)) {
		findings = append(findings, finding{
			Severity: severityInfo,
			Agent:    "codex",
			Scope:    scope,
			Check:    "project-trust",
			Path:     filepath.Dir(hooksPath),
			Message:  "Codex project hooks require this .codex layer to be trusted; if hooks do not run, approve/trust this project in Codex",
		})
	}

	if !exists(fs, hooksPath) {
		findings = append(findings, finding{
			Severity: severityInfo,
			Agent:    "codex",
			Scope:    scope,
			Check:    "config-exists",
			Path:     hooksPath,
			Message:  "Codex hooks file not found",
		})
		return findings
	}

	var hooks codexHooksFile
	if err := readJSON(fs, hooksPath, &hooks); err != nil {
		return append(findings, errorFinding("codex", scope, "hooks-parse", hooksPath, err.Error()))
	}
	findings = append(findings, finding{
		Severity: severityOK,
		Agent:    "codex",
		Scope:    scope,
		Check:    "hooks-parse",
		Path:     hooksPath,
		Message:  "Codex hooks file parsed",
	})

	entries := codexScutEntries(hooks)
	if len(entries) == 0 {
		findings = append(findings, finding{
			Severity: severityWarn,
			Agent:    "codex",
			Scope:    scope,
			Check:    "scut-hooks",
			Path:     hooksPath,
			Message:  "no scut Codex hooks found",
		})
	} else {
		findings = append(findings, finding{
			Severity: severityOK,
			Agent:    "codex",
			Scope:    scope,
			Check:    "scut-hooks",
			Path:     hooksPath,
			Message:  fmt.Sprintf("%d scut Codex hook entries found", len(entries)),
		})
	}
	return append(findings, inspectCommands(fs, "codex", scope, hooksPath, entries)...)
}

func inspectCodexConfig(fs afero.Fs, scope, path string, hooksJSONExists bool) []finding {
	if !exists(fs, path) {
		return nil
	}
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return []finding{errorFinding("codex", scope, "config-toml", path, err.Error())}
	}
	text := string(data)
	var findings []finding
	if hasTOMLSection(text, "hooks") && hooksJSONExists {
		findings = append(findings, finding{
			Severity: severityWarn,
			Agent:    "codex",
			Scope:    scope,
			Check:    "inline-hooks",
			Path:     path,
			Message:  "config.toml has inline [hooks] while hooks.json also exists; Codex merges both and warns",
		})
	}
	if tomlBoolValue(text, "hooks") == "false" || tomlBoolValue(text, "codex_hooks") == "false" {
		findings = append(findings, finding{
			Severity: severityError,
			Agent:    "codex",
			Scope:    scope,
			Check:    "hooks-feature",
			Path:     path,
			Message:  "Codex hooks appear to be disabled in config.toml",
		})
	}
	return findings
}

func inspectCommands(fs afero.Fs, agentName, scope, path string, commands []string) []finding {
	var findings []finding
	for _, command := range commands {
		fields := strings.Fields(command)
		if len(fields) == 0 {
			findings = append(findings, errorFinding(agentName, scope, "hook-command", path, "empty scut hook command"))
			continue
		}
		bin := fields[0]
		if bin == "scut" {
			continue
		}
		if strings.Contains(filepath.Base(bin), "scut") {
			if filepath.IsAbs(bin) && !exists(fs, bin) {
				findings = append(findings, finding{
					Severity: severityError,
					Agent:    agentName,
					Scope:    scope,
					Check:    "hook-command",
					Path:     path,
					Message:  fmt.Sprintf("hook command references missing scut binary %q", bin),
				})
				continue
			}
			findings = append(findings, finding{
				Severity: severityWarn,
				Agent:    agentName,
				Scope:    scope,
				Check:    "hook-command",
				Path:     path,
				Message:  fmt.Sprintf("hook command uses non-standard scut binary path %q", bin),
			})
		}
	}
	return findings
}

func readJSON(fs afero.Fs, path string, v any) error {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return fmt.Errorf("reading %q: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parsing %q: %w", path, err)
	}
	return nil
}

func claudeSettingsPath(scope string) (string, error) {
	switch scope {
	case "project":
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolving cwd: %w", err)
		}
		return filepath.Join(cwd, ".claude", "settings.json"), nil
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving user home directory: %w", err)
		}
		return filepath.Join(home, ".claude", "settings.json"), nil
	default:
		return "", fmt.Errorf("unknown scope %q", scope)
	}
}

func codexHooksPath(scope string) (string, error) {
	switch scope {
	case "project":
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolving cwd: %w", err)
		}
		return filepath.Join(cwd, ".codex", "hooks.json"), nil
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving user home directory: %w", err)
		}
		return filepath.Join(home, ".codex", "hooks.json"), nil
	default:
		return "", fmt.Errorf("unknown scope %q", scope)
	}
}

func codexConfigPath(scope string) (string, error) {
	switch scope {
	case "project":
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolving cwd: %w", err)
		}
		return filepath.Join(cwd, ".codex", "config.toml"), nil
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving user home directory: %w", err)
		}
		return filepath.Join(home, ".codex", "config.toml"), nil
	default:
		return "", fmt.Errorf("unknown scope %q", scope)
	}
}

func exists(fs afero.Fs, path string) bool {
	_, err := fs.Stat(path)
	return err == nil
}

func errorFinding(agentName, scope, check, path, message string) finding {
	return finding{
		Severity: severityError,
		Agent:    agentName,
		Scope:    scope,
		Check:    check,
		Path:     path,
		Message:  message,
	}
}

func hasError(findings []finding) bool {
	return slices.ContainsFunc(findings, func(f finding) bool {
		return f.Severity == severityError
	})
}

func writeJSON(w io.Writer, out output) error {
	data, err := json.Marshal(out, json.Deterministic(true))
	if err != nil {
		return fmt.Errorf("marshalling doctor JSON: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func writeHuman(w io.Writer, findings []finding) error {
	for _, f := range findings {
		label := strings.ToUpper(string(f.Severity))
		subject := f.Check
		if f.Agent != "" {
			subject = f.Agent + "/" + subject
		}
		if f.Scope != "" {
			subject = subject + " [" + f.Scope + "]"
		}
		if f.Path != "" {
			if _, err := fmt.Fprintf(w, "%-5s %-34s %s\n      %s\n", label, subject, f.Message, f.Path); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "%-5s %-34s %s\n", label, subject, f.Message); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasTOMLSection(text, section string) bool {
	target := "[" + section + "]"
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(stripTOMLComment(line))
		if line == target {
			return true
		}
	}
	return false
}

func tomlBoolValue(text, key string) string {
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(stripTOMLComment(line))
		before, after, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(before) != key {
			continue
		}
		value := strings.ToLower(strings.Trim(strings.TrimSpace(after), `"'`))
		if value == "true" || value == "false" {
			return value
		}
	}
	return ""
}

func stripTOMLComment(line string) string {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		return line[:i]
	}
	return line
}

type claudeSettings struct {
	StatusLine *struct {
		Command string `json:"command"`
	} `json:"statusLine"`
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func claudeScutEntries(s claudeSettings) ([]string, []string) {
	var hooks []string
	var statusLines []string
	if s.StatusLine != nil && ownsClaudeCommand(s.StatusLine.Command) {
		statusLines = append(statusLines, s.StatusLine.Command)
	}
	for _, groups := range s.Hooks {
		for _, group := range groups {
			for _, hook := range group.Hooks {
				if hook.Type == "command" && ownsClaudeCommand(hook.Command) {
					hooks = append(hooks, hook.Command)
				}
			}
		}
	}
	return hooks, statusLines
}

func ownsClaudeCommand(command string) bool {
	c := strings.TrimLeft(command, " \t")
	return c == "scut" || strings.HasPrefix(c, "scut ") || strings.HasPrefix(c, "scut\t")
}

type codexHooksFile struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func codexScutEntries(h codexHooksFile) []string {
	var commands []string
	for _, groups := range h.Hooks {
		for _, group := range groups {
			for _, hook := range group.Hooks {
				if hook.Type == "command" && ownsCodexCommand(hook.Command) {
					commands = append(commands, hook.Command)
				}
			}
		}
	}
	return commands
}

func ownsCodexCommand(command string) bool {
	c := strings.TrimLeft(command, " \t")
	return strings.HasPrefix(c, "scut codex hook ") ||
		strings.HasPrefix(c, "scut codex --log hook ") ||
		(strings.HasPrefix(c, "scut codex --log-level=") && strings.Contains(c, " hook "))
}

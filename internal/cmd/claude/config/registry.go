//go:build goexperiment.jsonv2

// Package config implements the "scut claude config" command group.
package config

// hookSpec describes one row in the install registry.
// Slug is the --only token AND the leaf command name under "scut claude hook".
// Event is Claude Code's event-name key in settings.json.
// Matcher defaults to "*" and StatusMessage to "".
type hookSpec struct {
	Slug          string
	Event         string
	Matcher       string
	StatusMessage string
}

// hookSpecs is the registry of installable hook events. It is initialised
// once at package load and never mutated thereafter — readers may rely on
// the slice and its element values being stable for the process lifetime.
//
// Slugs must exactly match the cmd:"" tag values on fields of hook.Cmd.
var hookSpecs = []hookSpec{
	{Slug: "session-start", Event: "SessionStart", Matcher: "*"},
	{Slug: "session-end", Event: "SessionEnd", Matcher: "*"},
	{Slug: "instructions-loaded", Event: "InstructionsLoaded", Matcher: "*"},
	{Slug: "user-prompt-submit", Event: "UserPromptSubmit", Matcher: "*"},
	{Slug: "pre-tool-use", Event: "PreToolUse", Matcher: "*"},
	{Slug: "post-tool-use", Event: "PostToolUse", Matcher: "Write|Edit", StatusMessage: "Formatting..."},
	{Slug: "post-tool-use-failure", Event: "PostToolUseFailure", Matcher: "*"},
	{Slug: "permission-request", Event: "PermissionRequest", Matcher: "*"},
	{Slug: "notification", Event: "Notification", Matcher: "*"},
	{Slug: "subagent-start", Event: "SubagentStart", Matcher: "*"},
	{Slug: "subagent-stop", Event: "SubagentStop", Matcher: "*"},
	{Slug: "stop", Event: "Stop", Matcher: "*"},
	{Slug: "stop-failure", Event: "StopFailure", Matcher: "*"},
	{Slug: "task-created", Event: "TaskCreated", Matcher: "*"},
	{Slug: "task-completed", Event: "TaskCompleted", Matcher: "*"},
	{Slug: "teammate-idle", Event: "TeammateIdle", Matcher: "*"},
	{Slug: "config-change", Event: "ConfigChange", Matcher: "*"},
	{Slug: "cwd-changed", Event: "CwdChanged", Matcher: "*"},
	{Slug: "file-changed", Event: "FileChanged", Matcher: "*"},
	{Slug: "worktree-create", Event: "WorktreeCreate", Matcher: "*"},
	{Slug: "worktree-remove", Event: "WorktreeRemove", Matcher: "*"},
	{Slug: "pre-compact", Event: "PreCompact", Matcher: "*"},
	{Slug: "post-compact", Event: "PostCompact", Matcher: "*"},
	{Slug: "elicitation", Event: "Elicitation", Matcher: "*"},
	{Slug: "elicitation-result", Event: "ElicitationResult", Matcher: "*"},
}

// hookSpecBySlug returns the hookSpec for the given slug, and reports whether
// the slug was found.
func hookSpecBySlug(slug string) (hookSpec, bool) {
	for _, s := range hookSpecs {
		if s.Slug == slug {
			return s, true
		}
	}
	return hookSpec{}, false
}

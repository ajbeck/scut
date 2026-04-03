// Package hook implements the "claude hook" subcommand tree.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	cc "github.com/ajbeck/botctrl/hooks/claudecode"
)

// ms returns milliseconds elapsed since start as an int64 for log attributes.
func ms(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

// Cmd is the Kong command group for "botctrl claude hook".
type Cmd struct {
	SessionStart       sessionStartCmd       `cmd:"session-start" help:"Inject context when a session begins or resumes."`
	SessionEnd         sessionEndCmd         `cmd:"session-end" help:"Record session termination."`
	InstructionsLoaded instructionsLoadedCmd `cmd:"instructions-loaded" help:"Record when a CLAUDE.md or rules file is loaded."`
	UserPromptSubmit   userPromptSubmitCmd   `cmd:"user-prompt-submit" help:"Validate or annotate user prompts before processing."`
	PreToolUse         preToolUseCmd         `cmd:"pre-tool-use" help:"Allow, deny, or modify tool calls before execution."`
	PostToolUse        postToolUseCmd        `cmd:"post-tool-use" help:"Format files after successful write or edit tool calls."`
	PostToolUseFailure postToolUseFailureCmd `cmd:"post-tool-use-failure" help:"Record context after a tool call fails."`
	PermissionRequest  permissionRequestCmd  `cmd:"permission-request" help:"Auto-approve or deny permission prompts."`
	Notification       notificationCmd       `cmd:"notification" help:"Record agent notifications."`
	SubagentStart      subagentStartCmd      `cmd:"subagent-start" help:"Inject context when a subagent is spawned."`
	SubagentStop       subagentStopCmd       `cmd:"subagent-stop" help:"Allow or block subagent termination."`
	Stop               stopCmd               `cmd:"stop" help:"Allow or block agent turn completion."`
	StopFailure        stopFailureCmd        `cmd:"stop-failure" help:"Record API errors that ended a turn."`
	TaskCreated        taskCreatedCmd        `cmd:"task-created" help:"Validate or block task creation."`
	TaskCompleted      taskCompletedCmd      `cmd:"task-completed" help:"Validate or block task completion."`
	TeammateIdle       teammateIdleCmd       `cmd:"teammate-idle" help:"Decide whether an idle teammate should continue."`
	ConfigChange       configChangeCmd       `cmd:"config-change" help:"Allow or block configuration changes."`
	CwdChanged         cwdChangedCmd         `cmd:"cwd-changed" help:"React to working directory changes."`
	FileChanged        fileChangedCmd        `cmd:"file-changed" help:"React to watched file changes on disk."`
	WorktreeCreate     worktreeCreateCmd     `cmd:"worktree-create" help:"Provide a custom worktree path."`
	WorktreeRemove     worktreeRemoveCmd     `cmd:"worktree-remove" help:"Record worktree removal."`
	PreCompact         preCompactCmd         `cmd:"pre-compact" help:"Record before context compaction begins."`
	PostCompact        postCompactCmd        `cmd:"post-compact" help:"Record after context compaction completes."`
	Elicitation        elicitationCmd        `cmd:"elicitation" help:"Accept, decline, or cancel MCP user input requests."`
	ElicitationResult  elicitationResultCmd  `cmd:"elicitation-result" help:"Validate or modify MCP elicitation responses."`
}

// writeJSON encodes v as JSON to w.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// ---------------------------------------------------------------------------
// SessionStart
// ---------------------------------------------------------------------------

type sessionStartCmd struct{}

func (c *sessionStartCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()
	var in cc.SessionStartInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding SessionStart input: %w", err)
	}
	logger.Info("handled", "hook", "session-start", "session_id", in.SessionID, "source", in.Source, "duration_ms", ms(start))
	return writeJSON(stdout, cc.SessionStartOutput{
		AdditionalContext: new("hello from botctrl session-start"),
	})
}

// ---------------------------------------------------------------------------
// SessionEnd
// ---------------------------------------------------------------------------

type sessionEndCmd struct{}

func (c *sessionEndCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()
	var in cc.SessionEndInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding SessionEnd input: %w", err)
	}
	logger.Info("handled", "hook", "session-end", "session_id", in.SessionID, "reason", in.Reason, "duration_ms", ms(start))
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// InstructionsLoaded
// ---------------------------------------------------------------------------

type instructionsLoadedCmd struct{}

func (c *instructionsLoadedCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.InstructionsLoadedInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding InstructionsLoaded input: %w", err)
	}
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// UserPromptSubmit
// ---------------------------------------------------------------------------

type userPromptSubmitCmd struct{}

func (c *userPromptSubmitCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.UserPromptSubmitInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding UserPromptSubmit input: %w", err)
	}
	return writeJSON(stdout, cc.UserPromptSubmitOutput{
		AdditionalContext: new("hello from botctrl user-prompt-submit"),
	})
}

// ---------------------------------------------------------------------------
// PreToolUse
// ---------------------------------------------------------------------------

type preToolUseCmd struct{}

func (c *preToolUseCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()
	var in cc.PreToolUseInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PreToolUse input: %w", err)
	}
	logger.Info("handled", "hook", "pre-tool-use", "session_id", in.SessionID, "tool_name", in.ToolName, "duration_ms", ms(start))
	return writeJSON(stdout, cc.PreToolUseOutput{
		HookSpecificOutput: cc.PreToolUseHookOutput{
			HookEventName:            cc.EventPreToolUse,
			PermissionDecision:       cc.PermissionAllow,
			PermissionDecisionReason: new("hello from botctrl pre-tool-use"),
		},
	})
}

// ---------------------------------------------------------------------------
// PostToolUseFailure
// ---------------------------------------------------------------------------

type postToolUseFailureCmd struct{}

func (c *postToolUseFailureCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.PostToolUseFailureInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PostToolUseFailure input: %w", err)
	}
	return writeJSON(stdout, cc.PostToolUseFailureOutput{
		AdditionalContext: new("hello from botctrl post-tool-use-failure"),
	})
}

// ---------------------------------------------------------------------------
// PermissionRequest
// ---------------------------------------------------------------------------

type permissionRequestCmd struct{}

func (c *permissionRequestCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.PermissionRequestInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PermissionRequest input: %w", err)
	}
	return writeJSON(stdout, cc.PermissionRequestOutput{
		HookSpecificOutput: cc.PermissionRequestHookOutput{
			HookEventName: cc.EventPermissionRequest,
			Decision: cc.PermissionRequestDecision{
				Behavior: cc.BehaviorAllow,
				Message:  new("hello from botctrl permission-request"),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Notification
// ---------------------------------------------------------------------------

type notificationCmd struct{}

func (c *notificationCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.NotificationInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding Notification input: %w", err)
	}
	return writeJSON(stdout, cc.NotificationOutput{
		AdditionalContext: new("hello from botctrl notification"),
	})
}

// ---------------------------------------------------------------------------
// SubagentStart
// ---------------------------------------------------------------------------

type subagentStartCmd struct{}

func (c *subagentStartCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.SubagentStartInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding SubagentStart input: %w", err)
	}
	return writeJSON(stdout, cc.SubagentStartOutput{
		AdditionalContext: new("hello from botctrl subagent-start"),
	})
}

// ---------------------------------------------------------------------------
// SubagentStop
// ---------------------------------------------------------------------------

type subagentStopCmd struct{}

func (c *subagentStopCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.SubagentStopInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding SubagentStop input: %w", err)
	}
	return writeJSON(stdout, cc.SubagentStopOutput{
		Reason: new("hello from botctrl subagent-stop"),
	})
}

// ---------------------------------------------------------------------------
// Stop
// ---------------------------------------------------------------------------

type stopCmd struct{}

func (c *stopCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.StopInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding Stop input: %w", err)
	}
	return writeJSON(stdout, cc.StopOutput{
		Reason: new("hello from botctrl stop"),
	})
}

// ---------------------------------------------------------------------------
// StopFailure
// ---------------------------------------------------------------------------

type stopFailureCmd struct{}

func (c *stopFailureCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()
	var in cc.StopFailureInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding StopFailure input: %w", err)
	}
	logger.Warn("handled", "hook", "stop-failure", "session_id", in.SessionID, "error", in.Error, "duration_ms", ms(start))
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// TaskCreated
// ---------------------------------------------------------------------------

type taskCreatedCmd struct{}

func (c *taskCreatedCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.TaskCreatedInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding TaskCreated input: %w", err)
	}
	return writeJSON(stdout, cc.TaskOutput{})
}

// ---------------------------------------------------------------------------
// TaskCompleted
// ---------------------------------------------------------------------------

type taskCompletedCmd struct{}

func (c *taskCompletedCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.TaskCompletedInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding TaskCompleted input: %w", err)
	}
	return writeJSON(stdout, cc.TaskOutput{})
}

// ---------------------------------------------------------------------------
// TeammateIdle
// ---------------------------------------------------------------------------

type teammateIdleCmd struct{}

func (c *teammateIdleCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.TeammateIdleInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding TeammateIdle input: %w", err)
	}
	return writeJSON(stdout, cc.TeammateIdleOutput{})
}

// ---------------------------------------------------------------------------
// ConfigChange
// ---------------------------------------------------------------------------

type configChangeCmd struct{}

func (c *configChangeCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.ConfigChangeInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding ConfigChange input: %w", err)
	}
	return writeJSON(stdout, cc.ConfigChangeOutput{})
}

// ---------------------------------------------------------------------------
// CwdChanged
// ---------------------------------------------------------------------------

type cwdChangedCmd struct{}

func (c *cwdChangedCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.CwdChangedInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding CwdChanged input: %w", err)
	}
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// FileChanged
// ---------------------------------------------------------------------------

type fileChangedCmd struct{}

func (c *fileChangedCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.FileChangedInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding FileChanged input: %w", err)
	}
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// WorktreeCreate
// ---------------------------------------------------------------------------

type worktreeCreateCmd struct{}

func (c *worktreeCreateCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.WorktreeCreateInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding WorktreeCreate input: %w", err)
	}
	return writeJSON(stdout, cc.WorktreeCreateOutput{
		HookSpecificOutput: cc.WorktreeCreateHookOutput{
			WorktreePath: new("/tmp/mock-worktree"),
		},
	})
}

// ---------------------------------------------------------------------------
// WorktreeRemove
// ---------------------------------------------------------------------------

type worktreeRemoveCmd struct{}

func (c *worktreeRemoveCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.WorktreeRemoveInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding WorktreeRemove input: %w", err)
	}
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// PreCompact
// ---------------------------------------------------------------------------

type preCompactCmd struct{}

func (c *preCompactCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()
	var in cc.PreCompactInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PreCompact input: %w", err)
	}
	logger.Info("handled", "hook", "pre-compact", "session_id", in.SessionID, "trigger", in.Trigger, "duration_ms", ms(start))
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// PostCompact
// ---------------------------------------------------------------------------

type postCompactCmd struct{}

func (c *postCompactCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()
	var in cc.PostCompactInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PostCompact input: %w", err)
	}
	logger.Info("handled", "hook", "post-compact", "session_id", in.SessionID, "trigger", in.Trigger, "duration_ms", ms(start))
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// Elicitation
// ---------------------------------------------------------------------------

type elicitationCmd struct{}

func (c *elicitationCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.ElicitationInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding Elicitation input: %w", err)
	}
	return writeJSON(stdout, cc.ElicitationOutput{
		HookSpecificOutput: cc.ElicitationHookOutput{
			HookEventName: cc.EventElicitation,
			Action:        cc.ElicitationAccept,
		},
	})
}

// ---------------------------------------------------------------------------
// ElicitationResult
// ---------------------------------------------------------------------------

type elicitationResultCmd struct{}

func (c *elicitationResultCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cc.ElicitationResultInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding ElicitationResult input: %w", err)
	}
	a := cc.ElicitationAccept
	return writeJSON(stdout, cc.ElicitationResultOutput{
		Action: &a,
	})
}

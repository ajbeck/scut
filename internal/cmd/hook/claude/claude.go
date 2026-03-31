// Package claude implements the "hook claude" subcommand tree.
package claude

import (
	"encoding/json"
	"fmt"
	"io"

	cc "github.com/ajbeck/botctrl/hooks/claudecode"
)

// Cmd is the Kong command group for "botctrl hook claude".
type Cmd struct {
	SessionStart       sessionStartCmd       `cmd:"session-start" help:"Handle SessionStart events."`
	SessionEnd         sessionEndCmd         `cmd:"session-end" help:"Handle SessionEnd events."`
	InstructionsLoaded instructionsLoadedCmd `cmd:"instructions-loaded" help:"Handle InstructionsLoaded events."`
	UserPromptSubmit   userPromptSubmitCmd   `cmd:"user-prompt-submit" help:"Handle UserPromptSubmit events."`
	PreToolUse         preToolUseCmd         `cmd:"pre-tool-use" help:"Handle PreToolUse events."`
	PostToolUse        postToolUseCmd        `cmd:"post-tool-use" help:"Handle PostToolUse events."`
	PostToolUseFailure postToolUseFailureCmd `cmd:"post-tool-use-failure" help:"Handle PostToolUseFailure events."`
	PermissionRequest  permissionRequestCmd  `cmd:"permission-request" help:"Handle PermissionRequest events."`
	Notification       notificationCmd       `cmd:"notification" help:"Handle Notification events."`
	SubagentStart      subagentStartCmd      `cmd:"subagent-start" help:"Handle SubagentStart events."`
	SubagentStop       subagentStopCmd       `cmd:"subagent-stop" help:"Handle SubagentStop events."`
	Stop               stopCmd               `cmd:"stop" help:"Handle Stop events."`
	StopFailure        stopFailureCmd        `cmd:"stop-failure" help:"Handle StopFailure events."`
	TaskCreated        taskCreatedCmd        `cmd:"task-created" help:"Handle TaskCreated events."`
	TaskCompleted      taskCompletedCmd      `cmd:"task-completed" help:"Handle TaskCompleted events."`
	TeammateIdle       teammateIdleCmd       `cmd:"teammate-idle" help:"Handle TeammateIdle events."`
	ConfigChange       configChangeCmd       `cmd:"config-change" help:"Handle ConfigChange events."`
	CwdChanged         cwdChangedCmd         `cmd:"cwd-changed" help:"Handle CwdChanged events."`
	FileChanged        fileChangedCmd        `cmd:"file-changed" help:"Handle FileChanged events."`
	WorktreeCreate     worktreeCreateCmd     `cmd:"worktree-create" help:"Handle WorktreeCreate events."`
	WorktreeRemove     worktreeRemoveCmd     `cmd:"worktree-remove" help:"Handle WorktreeRemove events."`
	PreCompact         preCompactCmd         `cmd:"pre-compact" help:"Handle PreCompact events."`
	PostCompact        postCompactCmd        `cmd:"post-compact" help:"Handle PostCompact events."`
	Elicitation        elicitationCmd        `cmd:"elicitation" help:"Handle Elicitation events."`
	ElicitationResult  elicitationResultCmd  `cmd:"elicitation-result" help:"Handle ElicitationResult events."`
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

func (c *sessionStartCmd) Run(stdin io.Reader, stdout io.Writer) error {
	var in cc.SessionStartInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding SessionStart input: %w", err)
	}
	return writeJSON(stdout, cc.SessionStartOutput{
		AdditionalContext: new("hello from botctrl session-start"),
	})
}

// ---------------------------------------------------------------------------
// SessionEnd
// ---------------------------------------------------------------------------

type sessionEndCmd struct{}

func (c *sessionEndCmd) Run(stdin io.Reader, stdout io.Writer) error {
	var in cc.SessionEndInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding SessionEnd input: %w", err)
	}
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// InstructionsLoaded
// ---------------------------------------------------------------------------

type instructionsLoadedCmd struct{}

func (c *instructionsLoadedCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *userPromptSubmitCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *preToolUseCmd) Run(stdin io.Reader, stdout io.Writer) error {
	var in cc.PreToolUseInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PreToolUse input: %w", err)
	}
	return writeJSON(stdout, cc.PreToolUseOutput{
		HookSpecificOutput: cc.PreToolUseHookOutput{
			HookEventName:            cc.EventPreToolUse,
			PermissionDecision:       cc.PermissionAllow,
			PermissionDecisionReason: new("hello from botctrl pre-tool-use"),
		},
	})
}

// ---------------------------------------------------------------------------
// PostToolUse
// ---------------------------------------------------------------------------

type postToolUseCmd struct{}

func (c *postToolUseCmd) Run(stdin io.Reader, stdout io.Writer) error {
	var in cc.PostToolUseInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PostToolUse input: %w", err)
	}
	return writeJSON(stdout, cc.PostToolUseOutput{
		AdditionalContext: new("hello from botctrl post-tool-use"),
	})
}

// ---------------------------------------------------------------------------
// PostToolUseFailure
// ---------------------------------------------------------------------------

type postToolUseFailureCmd struct{}

func (c *postToolUseFailureCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *permissionRequestCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *notificationCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *subagentStartCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *subagentStopCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *stopCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *stopFailureCmd) Run(stdin io.Reader, stdout io.Writer) error {
	var in cc.StopFailureInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding StopFailure input: %w", err)
	}
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// TaskCreated
// ---------------------------------------------------------------------------

type taskCreatedCmd struct{}

func (c *taskCreatedCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *taskCompletedCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *teammateIdleCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *configChangeCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *cwdChangedCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *fileChangedCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *worktreeCreateCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *worktreeRemoveCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *preCompactCmd) Run(stdin io.Reader, stdout io.Writer) error {
	var in cc.PreCompactInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PreCompact input: %w", err)
	}
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// PostCompact
// ---------------------------------------------------------------------------

type postCompactCmd struct{}

func (c *postCompactCmd) Run(stdin io.Reader, stdout io.Writer) error {
	var in cc.PostCompactInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PostCompact input: %w", err)
	}
	return writeJSON(stdout, cc.BaseOutput{})
}

// ---------------------------------------------------------------------------
// Elicitation
// ---------------------------------------------------------------------------

type elicitationCmd struct{}

func (c *elicitationCmd) Run(stdin io.Reader, stdout io.Writer) error {
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

func (c *elicitationResultCmd) Run(stdin io.Reader, stdout io.Writer) error {
	var in cc.ElicitationResultInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding ElicitationResult input: %w", err)
	}
	a := cc.ElicitationAccept
	return writeJSON(stdout, cc.ElicitationResultOutput{
		Action: &a,
	})
}

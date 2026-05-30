//go:build goexperiment.jsonv2

// Package hook implements the "codex hook" subcommand tree.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	cx "github.com/ajbeck/scut/hooks/codex"
)

func ms(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

type trailingArgs struct {
	Args []string `arg:"" optional:"" hidden:""`
}

// Cmd is the Kong command group for "scut codex hook".
type Cmd struct {
	SessionStart      sessionStartCmd      `cmd:"session-start" help:"Inject context when a Codex session begins or resumes."`
	SubagentStart     subagentStartCmd     `cmd:"subagent-start" help:"Inject context when a Codex subagent starts."`
	PreToolUse        preToolUseCmd        `cmd:"pre-tool-use" help:"Inspect supported tool calls before execution."`
	PermissionRequest permissionRequestCmd `cmd:"permission-request" help:"Allow, deny, or decline Codex approval requests."`
	PostToolUse       postToolUseCmd       `cmd:"post-tool-use" help:"Inspect supported tool calls after execution."`
	PreCompact        preCompactCmd        `cmd:"pre-compact" help:"React before Codex compacts the conversation."`
	PostCompact       postCompactCmd       `cmd:"post-compact" help:"React after Codex compacts the conversation."`
	UserPromptSubmit  userPromptSubmitCmd  `cmd:"user-prompt-submit" help:"Validate or annotate user prompts before processing."`
	SubagentStop      subagentStopCmd      `cmd:"subagent-stop" help:"Allow or continue Codex subagent termination."`
	Stop              stopCmd              `cmd:"stop" help:"Allow or continue Codex turn completion."`
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type sessionStartCmd struct{ trailingArgs }

func (c *sessionStartCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()
	var in cx.SessionStartInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding SessionStart input: %w", err)
	}
	logger.Info("handled", "hook", "session-start", "session_id", in.SessionID, "source", in.Source, "duration_ms", ms(start))
	return writeJSON(stdout, cx.SessionStartOutput{
		HookSpecificOutput: &cx.ContextHookOutput{
			HookEventName:     cx.EventSessionStart,
			AdditionalContext: new("hello from scut codex session-start"),
		},
	})
}

type subagentStartCmd struct{ trailingArgs }

func (c *subagentStartCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cx.SubagentStartInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding SubagentStart input: %w", err)
	}
	logger.Info("handled", "hook", "subagent-start", "session_id", in.SessionID, "agent_type", in.AgentType)
	return writeJSON(stdout, cx.SubagentStartOutput{
		HookSpecificOutput: &cx.ContextHookOutput{
			HookEventName:     cx.EventSubagentStart,
			AdditionalContext: new("hello from scut codex subagent-start"),
		},
	})
}

type preToolUseCmd struct{ trailingArgs }

func (c *preToolUseCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()
	var in cx.PreToolUseInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PreToolUse input: %w", err)
	}
	logger.Info("handled", "hook", "pre-tool-use", "session_id", in.SessionID, "turn_id", in.TurnID, "tool_name", in.ToolName, "duration_ms", ms(start))
	return writeJSON(stdout, cx.PreToolUseOutput{})
}

type permissionRequestCmd struct{ trailingArgs }

func (c *permissionRequestCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cx.PermissionRequestInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PermissionRequest input: %w", err)
	}
	logger.Info("handled", "hook", "permission-request", "session_id", in.SessionID, "turn_id", in.TurnID, "tool_name", in.ToolName)
	return writeJSON(stdout, cx.PermissionRequestOutput{})
}

type postToolUseCmd struct{ trailingArgs }

func (c *postToolUseCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	start := time.Now()
	var in cx.PostToolUseInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PostToolUse input: %w", err)
	}
	logger.Info("handled", "hook", "post-tool-use", "session_id", in.SessionID, "turn_id", in.TurnID, "tool_name", in.ToolName, "duration_ms", ms(start))
	return writeJSON(stdout, cx.PostToolUseOutput{})
}

type preCompactCmd struct{ trailingArgs }

func (c *preCompactCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cx.PreCompactInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PreCompact input: %w", err)
	}
	logger.Info("handled", "hook", "pre-compact", "session_id", in.SessionID, "turn_id", in.TurnID, "trigger", in.Trigger)
	return writeJSON(stdout, cx.BaseOutput{})
}

type postCompactCmd struct{ trailingArgs }

func (c *postCompactCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cx.PostCompactInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PostCompact input: %w", err)
	}
	logger.Info("handled", "hook", "post-compact", "session_id", in.SessionID, "turn_id", in.TurnID, "trigger", in.Trigger)
	return writeJSON(stdout, cx.BaseOutput{})
}

type userPromptSubmitCmd struct{ trailingArgs }

func (c *userPromptSubmitCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cx.UserPromptSubmitInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding UserPromptSubmit input: %w", err)
	}
	logger.Info("handled", "hook", "user-prompt-submit", "session_id", in.SessionID, "turn_id", in.TurnID)
	return writeJSON(stdout, cx.UserPromptSubmitOutput{
		HookSpecificOutput: &cx.ContextHookOutput{
			HookEventName:     cx.EventUserPromptSubmit,
			AdditionalContext: new("hello from scut codex user-prompt-submit"),
		},
	})
}

type subagentStopCmd struct{ trailingArgs }

func (c *subagentStopCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cx.SubagentStopInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding SubagentStop input: %w", err)
	}
	logger.Info("handled", "hook", "subagent-stop", "session_id", in.SessionID, "turn_id", in.TurnID, "agent_type", in.AgentType)
	return writeJSON(stdout, cx.SubagentStopOutput{})
}

type stopCmd struct{ trailingArgs }

func (c *stopCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
	var in cx.StopInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding Stop input: %w", err)
	}
	logger.Info("handled", "hook", "stop", "session_id", in.SessionID, "turn_id", in.TurnID)
	return writeJSON(stdout, cx.StopOutput{})
}

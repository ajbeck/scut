//go:build goexperiment.jsonv2

// Package codex provides types for Codex hook inputs and outputs.
//
// Codex invokes command hooks as subprocesses, passing a JSON payload on stdin
// and reading optional JSON from stdout. The types in this package model the
// documented command-hook payloads.
//
// See https://developers.openai.com/codex/hooks for the full specification.
package codex

import "encoding/json"

// EventName identifies which Codex hook event fired.
type EventName string

const (
	EventSessionStart      EventName = "SessionStart"
	EventSubagentStart     EventName = "SubagentStart"
	EventPreToolUse        EventName = "PreToolUse"
	EventPermissionRequest EventName = "PermissionRequest"
	EventPostToolUse       EventName = "PostToolUse"
	EventPreCompact        EventName = "PreCompact"
	EventPostCompact       EventName = "PostCompact"
	EventUserPromptSubmit  EventName = "UserPromptSubmit"
	EventSubagentStop      EventName = "SubagentStop"
	EventStop              EventName = "Stop"
)

// PermissionMode describes the active Codex permission mode.
type PermissionMode string

const (
	PermissionDefault           PermissionMode = "default"
	PermissionAcceptEdits       PermissionMode = "acceptEdits"
	PermissionPlan              PermissionMode = "plan"
	PermissionDontAsk           PermissionMode = "dontAsk"
	PermissionBypassPermissions PermissionMode = "bypassPermissions"
)

// SessionSource describes what triggered SessionStart.
type SessionSource string

const (
	SessionSourceStartup SessionSource = "startup"
	SessionSourceResume  SessionSource = "resume"
	SessionSourceClear   SessionSource = "clear"
	SessionSourceCompact SessionSource = "compact"
)

// CompactTrigger describes what initiated context compaction.
type CompactTrigger string

const (
	CompactManual CompactTrigger = "manual"
	CompactAuto   CompactTrigger = "auto"
)

// PermissionDecision is the hook-specific PreToolUse ruling.
type PermissionDecision string

const (
	PermissionAllow PermissionDecision = "allow"
	PermissionDeny  PermissionDecision = "deny"
	PermissionAsk   PermissionDecision = "ask"
)

// Decision is a top-level block decision used by several events.
type Decision string

const (
	DecisionBlock Decision = "block"
)

// PermissionBehavior is a PermissionRequest allow/deny ruling.
type PermissionBehavior string

const (
	BehaviorAllow PermissionBehavior = "allow"
	BehaviorDeny  PermissionBehavior = "deny"
)

// Input contains fields shared by Codex command hooks.
type Input struct {
	SessionID      string         `json:"session_id"`
	TranscriptPath *string        `json:"transcript_path,omitempty"`
	CWD            string         `json:"cwd"`
	HookEventName  EventName      `json:"hook_event_name"`
	Model          string         `json:"model"`
	PermissionMode PermissionMode `json:"permission_mode,omitempty"`
}

// TurnInput contains fields shared by turn-scoped hook events.
type TurnInput struct {
	Input
	TurnID string `json:"turn_id"`
}

// SessionStartInput is sent when a Codex session starts or resumes.
type SessionStartInput struct {
	Input
	Source SessionSource `json:"source"`
}

// SubagentStartInput is sent when a subagent starts.
type SubagentStartInput struct {
	TurnInput
	AgentID   string `json:"agent_id"`
	AgentType string `json:"agent_type"`
}

// PreToolUseInput is sent before a supported tool call executes.
type PreToolUseInput struct {
	TurnInput
	ToolName  string          `json:"tool_name"`
	ToolUseID string          `json:"tool_use_id"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// PermissionRequestInput is sent when Codex is about to ask for approval.
type PermissionRequestInput struct {
	TurnInput
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// PostToolUseInput is sent after a supported tool produces output.
type PostToolUseInput struct {
	TurnInput
	ToolName     string          `json:"tool_name"`
	ToolUseID    string          `json:"tool_use_id"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// PreCompactInput is sent before Codex compacts the conversation.
type PreCompactInput struct {
	TurnInput
	Trigger CompactTrigger `json:"trigger"`
}

// PostCompactInput is sent after Codex compacts the conversation.
type PostCompactInput struct {
	TurnInput
	Trigger CompactTrigger `json:"trigger"`
}

// UserPromptSubmitInput is sent before a prompt is submitted to Codex.
type UserPromptSubmitInput struct {
	TurnInput
	Prompt string `json:"prompt"`
}

// SubagentStopInput is sent when a subagent is about to stop.
type SubagentStopInput struct {
	TurnInput
	AgentID              string  `json:"agent_id"`
	AgentType            string  `json:"agent_type"`
	AgentTranscriptPath  *string `json:"agent_transcript_path,omitempty"`
	StopHookActive       bool    `json:"stop_hook_active"`
	LastAssistantMessage *string `json:"last_assistant_message,omitempty"`
}

// StopInput is sent when a Codex turn is about to stop.
type StopInput struct {
	TurnInput
	StopHookActive       bool    `json:"stop_hook_active"`
	LastAssistantMessage *string `json:"last_assistant_message,omitempty"`
}

// BaseOutput contains common JSON fields accepted by Codex hooks.
type BaseOutput struct {
	Continue       *bool   `json:"continue,omitempty"`
	StopReason     *string `json:"stopReason,omitempty"`
	SystemMessage  *string `json:"systemMessage,omitempty"`
	SuppressOutput *bool   `json:"suppressOutput,omitempty"`
}

// ContextHookOutput adds model-visible context for supported events.
type ContextHookOutput struct {
	HookEventName     EventName `json:"hookEventName"`
	AdditionalContext *string   `json:"additionalContext,omitempty"`
}

// SessionStartOutput is the response for SessionStart.
type SessionStartOutput struct {
	BaseOutput
	HookSpecificOutput *ContextHookOutput `json:"hookSpecificOutput,omitempty"`
}

// SubagentStartOutput is the response for SubagentStart.
type SubagentStartOutput struct {
	BaseOutput
	HookSpecificOutput *ContextHookOutput `json:"hookSpecificOutput,omitempty"`
}

// UserPromptSubmitOutput is the response for UserPromptSubmit.
type UserPromptSubmitOutput struct {
	BaseOutput
	Decision           *Decision          `json:"decision,omitempty"`
	Reason             *string            `json:"reason,omitempty"`
	HookSpecificOutput *ContextHookOutput `json:"hookSpecificOutput,omitempty"`
}

// PreToolUseHookOutput contains PreToolUse-specific output fields.
type PreToolUseHookOutput struct {
	HookEventName            EventName          `json:"hookEventName"`
	PermissionDecision       PermissionDecision `json:"permissionDecision,omitempty"`
	PermissionDecisionReason *string            `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             json.RawMessage    `json:"updatedInput,omitempty"`
	AdditionalContext        *string            `json:"additionalContext,omitempty"`
}

// PreToolUseOutput is the response for PreToolUse.
type PreToolUseOutput struct {
	BaseOutput
	Decision           *Decision             `json:"decision,omitempty"`
	Reason             *string               `json:"reason,omitempty"`
	HookSpecificOutput *PreToolUseHookOutput `json:"hookSpecificOutput,omitempty"`
}

// PermissionRequestDecision is the hook's ruling on a permission prompt.
type PermissionRequestDecision struct {
	Behavior PermissionBehavior `json:"behavior"`
	Message  *string            `json:"message,omitempty"`
}

// PermissionRequestHookOutput contains PermissionRequest-specific fields.
type PermissionRequestHookOutput struct {
	HookEventName EventName                 `json:"hookEventName"`
	Decision      PermissionRequestDecision `json:"decision"`
}

// PermissionRequestOutput is the response for PermissionRequest.
type PermissionRequestOutput struct {
	BaseOutput
	HookSpecificOutput *PermissionRequestHookOutput `json:"hookSpecificOutput,omitempty"`
}

// PostToolUseOutput is the response for PostToolUse.
type PostToolUseOutput struct {
	BaseOutput
	Decision           *Decision          `json:"decision,omitempty"`
	Reason             *string            `json:"reason,omitempty"`
	HookSpecificOutput *ContextHookOutput `json:"hookSpecificOutput,omitempty"`
}

// SubagentStopOutput is the response for SubagentStop.
type SubagentStopOutput struct {
	BaseOutput
	Decision *Decision `json:"decision,omitempty"`
	Reason   *string   `json:"reason,omitempty"`
}

// StopOutput is the response for Stop.
type StopOutput struct {
	BaseOutput
	Decision *Decision `json:"decision,omitempty"`
	Reason   *string   `json:"reason,omitempty"`
}

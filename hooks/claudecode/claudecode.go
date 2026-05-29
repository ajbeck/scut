// Package claudecode provides types for Claude Code hook inputs and outputs.
//
// Claude Code invokes hooks as subprocesses, passing a JSON payload on stdin
// and reading a JSON response from stdout. The types in this package model
// those payloads for every hook event type.
//
// See https://docs.anthropic.com/en/docs/claude-code/hooks for the full specification.
package claudecode

import "encoding/json"

// ---------------------------------------------------------------------------
// Typed string enums
// ---------------------------------------------------------------------------

// EventName identifies which hook event fired.
type EventName string

const (
	EventSessionStart       EventName = "SessionStart"
	EventSessionEnd         EventName = "SessionEnd"
	EventInstructionsLoaded EventName = "InstructionsLoaded"
	EventUserPromptSubmit   EventName = "UserPromptSubmit"
	EventPreToolUse         EventName = "PreToolUse"
	EventPostToolUse        EventName = "PostToolUse"
	EventPostToolUseFailure EventName = "PostToolUseFailure"
	EventPermissionRequest  EventName = "PermissionRequest"
	EventNotification       EventName = "Notification"
	EventSubagentStart      EventName = "SubagentStart"
	EventSubagentStop       EventName = "SubagentStop"
	EventStop               EventName = "Stop"
	EventStopFailure        EventName = "StopFailure"
	EventTaskCreated        EventName = "TaskCreated"
	EventTaskCompleted      EventName = "TaskCompleted"
	EventTeammateIdle       EventName = "TeammateIdle"
	EventConfigChange       EventName = "ConfigChange"
	EventCwdChanged         EventName = "CwdChanged"
	EventFileChanged        EventName = "FileChanged"
	EventWorktreeCreate     EventName = "WorktreeCreate"
	EventWorktreeRemove     EventName = "WorktreeRemove"
	EventPreCompact         EventName = "PreCompact"
	EventPostCompact        EventName = "PostCompact"
	EventElicitation        EventName = "Elicitation"
	EventElicitationResult  EventName = "ElicitationResult"
)

// PermissionMode describes the active permission mode for the session.
type PermissionMode string

const (
	PermissionDefault           PermissionMode = "default"
	PermissionPlan              PermissionMode = "plan"
	PermissionAcceptEdits       PermissionMode = "acceptEdits"
	PermissionAuto              PermissionMode = "auto"
	PermissionDontAsk           PermissionMode = "dontAsk"
	PermissionBypassPermissions PermissionMode = "bypassPermissions"
)

// SessionSource describes what triggered a SessionStart event.
type SessionSource string

const (
	SessionSourceStartup SessionSource = "startup"
	SessionSourceResume  SessionSource = "resume"
	SessionSourceClear   SessionSource = "clear"
	SessionSourceCompact SessionSource = "compact"
)

// SessionEndReason describes why a session ended.
type SessionEndReason string

const (
	SessionEndClear                     SessionEndReason = "clear"
	SessionEndResume                    SessionEndReason = "resume"
	SessionEndLogout                    SessionEndReason = "logout"
	SessionEndPromptInputExit           SessionEndReason = "prompt_input_exit"
	SessionEndBypassPermissionsDisabled SessionEndReason = "bypass_permissions_disabled"
	SessionEndOther                     SessionEndReason = "other"
)

// NotificationType categorizes a notification event.
type NotificationType string

const (
	NotificationPermissionPrompt  NotificationType = "permission_prompt"
	NotificationIdlePrompt        NotificationType = "idle_prompt"
	NotificationAuthSuccess       NotificationType = "auth_success"
	NotificationElicitationDialog NotificationType = "elicitation_dialog"
)

// MemoryType describes the origin of a loaded instructions file.
type MemoryType string

const (
	MemoryUser    MemoryType = "User"
	MemoryProject MemoryType = "Project"
	MemoryLocal   MemoryType = "Local"
	MemoryManaged MemoryType = "Managed"
)

// LoadReason describes why an instructions file was loaded.
type LoadReason string

const (
	LoadReasonSessionStart    LoadReason = "session_start"
	LoadReasonNestedTraversal LoadReason = "nested_traversal"
	LoadReasonPathGlobMatch   LoadReason = "path_glob_match"
	LoadReasonInclude         LoadReason = "include"
	LoadReasonCompact         LoadReason = "compact"
)

// PermissionDecision is the hook's ruling on a tool call.
type PermissionDecision string

const (
	PermissionAllow PermissionDecision = "allow"
	PermissionDeny  PermissionDecision = "deny"
	PermissionAsk   PermissionDecision = "ask"
)

// StopError categorizes the failure in a StopFailure event.
type StopError string

const (
	StopErrorRateLimit            StopError = "rate_limit"
	StopErrorAuthenticationFailed StopError = "authentication_failed"
	StopErrorBillingError         StopError = "billing_error"
	StopErrorInvalidRequest       StopError = "invalid_request"
	StopErrorServerError          StopError = "server_error"
	StopErrorMaxOutputTokens      StopError = "max_output_tokens"
	StopErrorUnknown              StopError = "unknown"
)

// ConfigSource identifies which configuration layer changed.
type ConfigSource string

const (
	ConfigUserSettings    ConfigSource = "user_settings"
	ConfigProjectSettings ConfigSource = "project_settings"
	ConfigLocalSettings   ConfigSource = "local_settings"
	ConfigPolicySettings  ConfigSource = "policy_settings"
	ConfigSkills          ConfigSource = "skills"
)

// FileChangeType describes how a watched file changed.
type FileChangeType string

const (
	FileChangeCreate FileChangeType = "create"
	FileChangeModify FileChangeType = "modify"
	FileChangeDelete FileChangeType = "delete"
)

// CompactTrigger describes what initiated context compaction.
type CompactTrigger string

const (
	CompactManual CompactTrigger = "manual"
	CompactAuto   CompactTrigger = "auto"
)

// ElicitationAction is the hook's ruling on an MCP elicitation.
type ElicitationAction string

const (
	ElicitationAccept  ElicitationAction = "accept"
	ElicitationDecline ElicitationAction = "decline"
	ElicitationCancel  ElicitationAction = "cancel"
)

// Decision is a general-purpose block/allow decision used by several output types.
type Decision string

const (
	DecisionBlock Decision = "block"
)

// PermissionSuggestionType categorizes a suggested permission change.
type PermissionSuggestionType string

const (
	SuggestionAddRules          PermissionSuggestionType = "addRules"
	SuggestionReplaceRules      PermissionSuggestionType = "replaceRules"
	SuggestionRemoveRules       PermissionSuggestionType = "removeRules"
	SuggestionSetMode           PermissionSuggestionType = "setMode"
	SuggestionAddDirectories    PermissionSuggestionType = "addDirectories"
	SuggestionRemoveDirectories PermissionSuggestionType = "removeDirectories"
)

// PermissionBehavior describes the intended behavior of a permission rule.
type PermissionBehavior string

const (
	BehaviorAllow PermissionBehavior = "allow"
	BehaviorDeny  PermissionBehavior = "deny"
	BehaviorAsk   PermissionBehavior = "ask"
)

// PermissionDestination identifies which settings layer to persist a permission change to.
type PermissionDestination string

const (
	DestinationSession         PermissionDestination = "session"
	DestinationLocalSettings   PermissionDestination = "localSettings"
	DestinationProjectSettings PermissionDestination = "projectSettings"
	DestinationUserSettings    PermissionDestination = "userSettings"
)

// ---------------------------------------------------------------------------
// Common input base
// ---------------------------------------------------------------------------

// Input contains fields present on every hook invocation.
type Input struct {
	SessionID      string         `json:"session_id"`
	TranscriptPath string         `json:"transcript_path"`
	CWD            string         `json:"cwd"`
	HookEventName  EventName      `json:"hook_event_name"`
	PermissionMode PermissionMode `json:"permission_mode"`
	AgentID        string         `json:"agent_id,omitempty"`
	AgentType      string         `json:"agent_type,omitempty"`
}

// ---------------------------------------------------------------------------
// Event-specific inputs
// ---------------------------------------------------------------------------

// SessionStartInput is sent when a session begins or resumes.
type SessionStartInput struct {
	Input
	Source SessionSource `json:"source"`
	Model  string        `json:"model"`
}

// SessionEndInput is sent when a session terminates.
type SessionEndInput struct {
	Input
	Reason SessionEndReason `json:"reason"`
}

// InstructionsLoadedInput is sent when a CLAUDE.md or rules file is loaded.
type InstructionsLoadedInput struct {
	Input
	FilePath        string     `json:"file_path"`
	MemoryType      MemoryType `json:"memory_type"`
	LoadReason      LoadReason `json:"load_reason"`
	Globs           []string   `json:"globs,omitempty"`
	TriggerFilePath string     `json:"trigger_file_path,omitempty"`
	ParentFilePath  string     `json:"parent_file_path,omitempty"`
}

// UserPromptSubmitInput is sent when the user submits a prompt.
type UserPromptSubmitInput struct {
	Input
	Prompt string `json:"prompt"`
}

// PreToolUseInput is sent before a tool call executes.
type PreToolUseInput struct {
	Input
	ToolName  string          `json:"tool_name"`
	ToolUseID string          `json:"tool_use_id"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// PostToolUseInput is sent after a tool call succeeds.
type PostToolUseInput struct {
	Input
	ToolName     string          `json:"tool_name"`
	ToolUseID    string          `json:"tool_use_id"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// FilePath extracts the file_path field from ToolInput.
// Returns an empty string if the field is absent, empty, or not a string.
func (in *PostToolUseInput) FilePath() string {
	if len(in.ToolInput) == 0 {
		return ""
	}
	var ti struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(in.ToolInput, &ti); err != nil {
		return ""
	}
	return ti.FilePath
}

// PostToolUseFailureInput is sent after a tool call fails.
type PostToolUseFailureInput struct {
	Input
	ToolName    string          `json:"tool_name"`
	ToolUseID   string          `json:"tool_use_id"`
	ToolInput   json.RawMessage `json:"tool_input"`
	Error       string          `json:"error"`
	IsInterrupt *bool           `json:"is_interrupt,omitempty"`
}

// PermissionRule describes a single permission rule in a suggestion.
type PermissionRule struct {
	ToolName    string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

// PermissionSuggestion is a proposed permission change offered with a PermissionRequest.
type PermissionSuggestion struct {
	Type        PermissionSuggestionType `json:"type"`
	Rules       []PermissionRule         `json:"rules,omitempty"`
	Behavior    PermissionBehavior       `json:"behavior,omitempty"`
	Destination PermissionDestination    `json:"destination,omitempty"`
}

// PermissionRequestInput is sent when a permission dialog is about to appear.
type PermissionRequestInput struct {
	Input
	ToolName              string                 `json:"tool_name"`
	ToolInput             json.RawMessage        `json:"tool_input"`
	PermissionSuggestions []PermissionSuggestion `json:"permission_suggestions"`
}

// NotificationInput is sent when Claude Code emits a notification.
type NotificationInput struct {
	Input
	Message          string           `json:"message"`
	Title            string           `json:"title,omitempty"`
	NotificationType NotificationType `json:"notification_type"`
}

// SubagentStartInput is sent when a subagent is spawned.
type SubagentStartInput struct {
	Input
}

// SubagentStopInput is sent when a subagent finishes.
type SubagentStopInput struct {
	Input
	StopHookActive       *bool  `json:"stop_hook_active"`
	AgentTranscriptPath  string `json:"agent_transcript_path"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

// StopInput is sent when Claude finishes responding.
type StopInput struct {
	Input
	StopHookActive       *bool  `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

// StopFailureInput is sent when a turn ends due to an API error.
type StopFailureInput struct {
	Input
	Error                StopError `json:"error"`
	ErrorDetails         string    `json:"error_details,omitempty"`
	LastAssistantMessage string    `json:"last_assistant_message"`
}

// TaskCreatedInput is sent when a task is created.
type TaskCreatedInput struct {
	Input
	TaskID          string `json:"task_id"`
	TaskSubject     string `json:"task_subject"`
	TaskDescription string `json:"task_description,omitempty"`
	TeammateName    string `json:"teammate_name,omitempty"`
	TeamName        string `json:"team_name,omitempty"`
}

// TaskCompletedInput is sent when a task is marked as completed.
type TaskCompletedInput struct {
	Input
	TaskID          string `json:"task_id"`
	TaskSubject     string `json:"task_subject"`
	TaskDescription string `json:"task_description,omitempty"`
	TeammateName    string `json:"teammate_name,omitempty"`
	TeamName        string `json:"team_name,omitempty"`
}

// TeammateIdleInput is sent when a teammate is about to go idle.
type TeammateIdleInput struct {
	Input
	TeammateName string `json:"teammate_name"`
	TeamName     string `json:"team_name"`
}

// ConfigChangeInput is sent when a configuration file changes.
type ConfigChangeInput struct {
	Input
	Source   ConfigSource `json:"source"`
	FilePath string       `json:"file_path,omitempty"`
}

// CwdChangedInput is sent when the working directory changes.
type CwdChangedInput struct {
	Input
	NewCWD      string `json:"new_cwd"`
	PreviousCWD string `json:"previous_cwd"`
}

// FileChangedInput is sent when a watched file changes on disk.
type FileChangedInput struct {
	Input
	FilePath    string         `json:"file_path"`
	ChangedType FileChangeType `json:"changed_type"`
}

// WorktreeCreateInput is sent when a worktree is being created.
type WorktreeCreateInput struct {
	Input
	WorktreeName string `json:"worktree_name"`
}

// WorktreeRemoveInput is sent when a worktree is being removed.
type WorktreeRemoveInput struct {
	Input
	WorktreePath string `json:"worktree_path"`
}

// PreCompactInput is sent before context compaction.
type PreCompactInput struct {
	Input
	Trigger CompactTrigger `json:"trigger"`
}

// PostCompactInput is sent after context compaction completes.
type PostCompactInput struct {
	Input
	Trigger CompactTrigger `json:"trigger"`
}

// ElicitationInput is sent when an MCP server requests user input.
type ElicitationInput struct {
	Input
	MCPServerName string          `json:"mcp_server_name"`
	FormSchema    json.RawMessage `json:"form_schema"`
}

// ElicitationResultInput is sent after the user responds to an MCP elicitation.
type ElicitationResultInput struct {
	Input
	MCPServerName string          `json:"mcp_server_name"`
	UserResponse  json.RawMessage `json:"user_response"`
}

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

// BaseOutput contains fields that any hook response may include.
type BaseOutput struct {
	Continue       *bool   `json:"continue,omitempty"`
	StopReason     *string `json:"stopReason,omitempty"`
	SuppressOutput *bool   `json:"suppressOutput,omitempty"`
	SystemMessage  *string `json:"systemMessage,omitempty"`
}

// SessionStartOutput is the response for a SessionStart hook.
type SessionStartOutput struct {
	BaseOutput
	AdditionalContext *string `json:"additionalContext,omitempty"`
}

// UserPromptSubmitOutput is the response for a UserPromptSubmit hook.
type UserPromptSubmitOutput struct {
	BaseOutput
	Decision          *Decision `json:"decision,omitempty"`
	Reason            *string   `json:"reason,omitempty"`
	AdditionalContext *string   `json:"additionalContext,omitempty"`
}

// PreToolUseHookOutput contains PreToolUse-specific output fields.
type PreToolUseHookOutput struct {
	HookEventName            EventName          `json:"hookEventName"`
	PermissionDecision       PermissionDecision `json:"permissionDecision"`
	PermissionDecisionReason *string            `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             json.RawMessage    `json:"updatedInput,omitempty"`
	AdditionalContext        *string            `json:"additionalContext,omitempty"`
}

// PreToolUseOutput is the response for a PreToolUse hook.
type PreToolUseOutput struct {
	BaseOutput
	HookSpecificOutput PreToolUseHookOutput `json:"hookSpecificOutput"`
}

// PostToolUseHookOutput contains PostToolUse-specific output fields.
type PostToolUseHookOutput struct {
	HookEventName     EventName       `json:"hookEventName"`
	AdditionalContext *string         `json:"additionalContext,omitempty"`
	UpdatedToolOutput json.RawMessage `json:"updatedToolOutput,omitempty"`
}

// PostToolUseOutput is the response for a PostToolUse hook.
type PostToolUseOutput struct {
	BaseOutput
	Decision             *Decision              `json:"decision,omitempty"`
	Reason               *string                `json:"reason,omitempty"`
	AdditionalContext    *string                `json:"additionalContext,omitempty"`
	UpdatedMCPToolOutput json.RawMessage        `json:"updatedMCPToolOutput,omitempty"`
	HookSpecificOutput   *PostToolUseHookOutput `json:"hookSpecificOutput,omitempty"`
}

// PostToolUseFailureOutput is the response for a PostToolUseFailure hook.
type PostToolUseFailureOutput struct {
	BaseOutput
	AdditionalContext *string `json:"additionalContext,omitempty"`
}

// PermissionUpdate describes a permission change to apply.
type PermissionUpdate struct {
	Type        PermissionSuggestionType `json:"type"`
	Mode        *string                  `json:"mode,omitempty"`
	Rules       []PermissionRule         `json:"rules,omitempty"`
	Destination PermissionDestination    `json:"destination"`
}

// PermissionRequestDecision is the hook's ruling on a permission request.
type PermissionRequestDecision struct {
	Behavior           PermissionBehavior `json:"behavior"`
	UpdatedInput       json.RawMessage    `json:"updatedInput,omitempty"`
	UpdatedPermissions []PermissionUpdate `json:"updatedPermissions,omitempty"`
	Message            *string            `json:"message,omitempty"`
	Interrupt          *bool              `json:"interrupt,omitempty"`
}

// PermissionRequestHookOutput contains PermissionRequest-specific output fields.
type PermissionRequestHookOutput struct {
	HookEventName EventName                 `json:"hookEventName"`
	Decision      PermissionRequestDecision `json:"decision"`
}

// PermissionRequestOutput is the response for a PermissionRequest hook.
type PermissionRequestOutput struct {
	BaseOutput
	HookSpecificOutput PermissionRequestHookOutput `json:"hookSpecificOutput"`
}

// NotificationOutput is the response for a Notification hook.
type NotificationOutput struct {
	BaseOutput
	AdditionalContext *string `json:"additionalContext,omitempty"`
}

// SubagentStartOutput is the response for a SubagentStart hook.
type SubagentStartOutput struct {
	BaseOutput
	AdditionalContext *string `json:"additionalContext,omitempty"`
}

// SubagentStopOutput is the response for a SubagentStop hook.
type SubagentStopOutput struct {
	BaseOutput
	Decision *Decision `json:"decision,omitempty"`
	Reason   *string   `json:"reason,omitempty"`
}

// StopOutput is the response for a Stop hook.
type StopOutput struct {
	BaseOutput
	Decision *Decision `json:"decision,omitempty"`
	Reason   *string   `json:"reason,omitempty"`
}

// TaskOutput is the response for TaskCreated and TaskCompleted hooks.
type TaskOutput struct {
	BaseOutput
}

// TeammateIdleOutput is the response for a TeammateIdle hook.
type TeammateIdleOutput struct {
	BaseOutput
}

// ConfigChangeOutput is the response for a ConfigChange hook.
type ConfigChangeOutput struct {
	BaseOutput
	Decision *Decision `json:"decision,omitempty"`
}

// WorktreeCreateHookOutput contains WorktreeCreate-specific output fields.
type WorktreeCreateHookOutput struct {
	WorktreePath *string `json:"worktreePath,omitempty"`
}

// WorktreeCreateOutput is the response for a WorktreeCreate hook.
type WorktreeCreateOutput struct {
	BaseOutput
	HookSpecificOutput WorktreeCreateHookOutput `json:"hookSpecificOutput,omitzero"`
}

// ElicitationHookOutput contains Elicitation-specific output fields.
type ElicitationHookOutput struct {
	HookEventName EventName         `json:"hookEventName"`
	Action        ElicitationAction `json:"action"`
	Content       json.RawMessage   `json:"content,omitempty"`
}

// ElicitationOutput is the response for an Elicitation hook.
type ElicitationOutput struct {
	BaseOutput
	HookSpecificOutput ElicitationHookOutput `json:"hookSpecificOutput"`
}

// ElicitationResultOutput is the response for an ElicitationResult hook.
type ElicitationResultOutput struct {
	BaseOutput
	Action  *ElicitationAction `json:"action,omitempty"`
	Content json.RawMessage    `json:"content,omitempty"`
}

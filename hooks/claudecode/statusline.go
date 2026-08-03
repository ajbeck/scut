package claudecode

// StatusLineInput is the JSON payload Claude Code pipes to the status line command.
// Unlike hook inputs, this is not an event — it is a periodic snapshot of the
// session state, sent after each assistant message (debounced at 300ms).
//
// See https://code.claude.com/docs/en/statusline for the full specification.
type StatusLineInput struct {
	CWD            string                 `json:"cwd"`
	SessionID      string                 `json:"session_id"`
	SessionName    *string                `json:"session_name,omitempty"`
	PromptID       *string                `json:"prompt_id,omitempty"`
	TranscriptPath string                 `json:"transcript_path"`
	Version        string                 `json:"version"`
	Model          StatusLineModel        `json:"model"`
	Workspace      StatusLineWorkspace    `json:"workspace"`
	Cost           StatusLineCost         `json:"cost"`
	ContextWindow  StatusLineContext      `json:"context_window"`
	Exceeds200K    bool                   `json:"exceeds_200k_tokens"`
	FastMode       bool                   `json:"fast_mode"`
	Effort         *StatusLineEffort      `json:"effort,omitempty"`
	Thinking       *StatusLineThinking    `json:"thinking,omitempty"`
	RateLimits     *StatusLineRateLimits  `json:"rate_limits,omitempty"`
	OutputStyle    *StatusLineOutputStyle `json:"output_style,omitempty"`
	Vim            *StatusLineVim         `json:"vim,omitempty"`
	Agent          *StatusLineAgent       `json:"agent,omitempty"`
	PR             *StatusLinePR          `json:"pr,omitempty"`
	Worktree       *StatusLineWorktree    `json:"worktree,omitempty"`
}

// StatusLineModel identifies the active model.
type StatusLineModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// StatusLineWorkspace carries directory context.
type StatusLineWorkspace struct {
	CurrentDir  string              `json:"current_dir"`
	ProjectDir  string              `json:"project_dir"`
	AddedDirs   []string            `json:"added_dirs"`
	GitWorktree *string             `json:"git_worktree,omitempty"`
	Repo        *StatusLineRepoInfo `json:"repo,omitempty"`
}

// StatusLineRepoInfo identifies the repository parsed from the origin remote.
type StatusLineRepoInfo struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

// StatusLineCost tracks session cost and duration.
type StatusLineCost struct {
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMS    int64   `json:"total_duration_ms"`
	TotalAPIDurationMS int64   `json:"total_api_duration_ms"`
	TotalLinesAdded    int     `json:"total_lines_added"`
	TotalLinesRemoved  int     `json:"total_lines_removed"`
}

// StatusLineContextUsage holds token counts from the most recent API call.
type StatusLineContextUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// StatusLineContext tracks context window consumption.
type StatusLineContext struct {
	TotalInputTokens    int                     `json:"total_input_tokens"`
	TotalOutputTokens   int                     `json:"total_output_tokens"`
	ContextWindowSize   int                     `json:"context_window_size"`
	UsedPercentage      *float64                `json:"used_percentage"`
	RemainingPercentage *float64                `json:"remaining_percentage"`
	CurrentUsage        *StatusLineContextUsage `json:"current_usage"`
}

// StatusLineEffort holds the active reasoning effort when the model supports it.
type StatusLineEffort struct {
	Level string `json:"level"`
}

// StatusLineThinking holds the active extended-thinking state.
type StatusLineThinking struct {
	Enabled bool `json:"enabled"`
}

// StatusLineRateWindow holds usage data for a single rate limit window.
type StatusLineRateWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

// StatusLineRateLimits holds rate limit windows. Only present for Claude.ai subscribers.
type StatusLineRateLimits struct {
	FiveHour *StatusLineRateWindow `json:"five_hour,omitempty"`
	SevenDay *StatusLineRateWindow `json:"seven_day,omitempty"`
}

// StatusLineOutputStyle holds the current output style.
type StatusLineOutputStyle struct {
	Name string `json:"name"`
}

// StatusLineVim holds vim mode state. Only present when vim mode is enabled.
type StatusLineVim struct {
	Mode string `json:"mode"`
}

// StatusLineAgent holds agent info. Only present with --agent flag.
type StatusLineAgent struct {
	Name string `json:"name"`
}

// StatusLinePR holds the open pull request associated with the current branch.
type StatusLinePR struct {
	Number      int     `json:"number"`
	URL         string  `json:"url"`
	ReviewState *string `json:"review_state,omitempty"`
}

// StatusLineWorktree holds worktree info. Only present during --worktree sessions.
type StatusLineWorktree struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Branch         string `json:"branch,omitempty"`
	OriginalCWD    string `json:"original_cwd,omitempty"`
	OriginalBranch string `json:"original_branch,omitempty"`
}

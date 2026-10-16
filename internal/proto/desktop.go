package proto

import "time"

type PendingRequests struct {
	Permissions []PermissionRequest `json:"permissions"`
	Question    *QuestionRequest    `json:"question"`
}

type DesktopCapabilities struct {
	ProtocolVersion        int    `json:"protocol_version"`
	RunRegistry            bool   `json:"run_registry"`
	RunRegistryPersistence string `json:"run_registry_persistence"`
	PendingRequests        bool   `json:"pending_requests"`
	EventReplay            bool   `json:"event_replay"`
	ModelCapabilities      bool   `json:"model_capabilities"`
	Plugins                bool   `json:"plugins"`
	VoiceUpload            bool   `json:"voice_upload"`
}

type RunRecord struct {
	RunID      string     `json:"run_id"`
	SessionID  string     `json:"session_id"`
	Status     string     `json:"status"`
	AcceptedAt time.Time  `json:"accepted_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
	MessageID  string     `json:"message_id,omitempty"`
	Text       string     `json:"text,omitempty"`
}

type ModelCapabilities struct {
	ProviderID          string   `json:"provider_id"`
	ModelID             string   `json:"model_id"`
	Name                string   `json:"name"`
	CanReason           bool     `json:"can_reason"`
	ReasoningModes      []string `json:"reasoning_modes"`
	ReasoningLevels     []string `json:"reasoning_levels"`
	SupportsAttachments bool     `json:"supports_attachments"`
	ContextWindow       int64    `json:"context_window"`
	DefaultMaxTokens    int64    `json:"default_max_tokens"`
}

type BackgroundJob struct {
	SessionID   string `json:"session_id"`
	ID          string `json:"id"`
	Command     string `json:"command"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	Error       string `json:"error,omitempty"`
}

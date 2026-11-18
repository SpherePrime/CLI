package customagents

import "context"

const (
	RoleOrchestrator = "orchestrator"
	RoleWorker       = "worker"
	RoleVerifier     = "verifier"

	TriggerManual   = "manual"
	TriggerInterval = "interval"
	TriggerCron     = "cron"
	TriggerEvent    = "event"
	TriggerMonitor  = "monitor"

	MonitorCommand = "command"
	MonitorHTTP    = "http"
	MonitorFile    = "file"

	StatusIdle       = "idle"
	StatusMonitoring = "monitoring"
	StatusRunning    = "running"
	StatusVerifying  = "verifying"
	StatusRework     = "rework"
	StatusPaused     = "paused"
	StatusError      = "error"

	RunRunning         = "running"
	RunVerifying       = "verifying"
	RunRework          = "rework"
	RunPassed          = "passed"
	RunFailed          = "failed"
	RunSkippedNoChange = "skipped_no_change"
	RunCancelled       = "cancelled"

	ModelModeAuto   = "auto"
	ModelModeManual = "manual"

	CheckCommand    = "command"
	CheckRegex      = "regex"
	CheckJSONFields = "json_fields"
	CheckFileExists = "file_exists"

	EventWorkflowUpdated = "workflow_updated"
	EventWorkflowDeleted = "workflow_deleted"
)

type DeterministicCheck struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Target   string `json:"target"`
	Pattern  string `json:"pattern"`
	Expected string `json:"expected"`
	Required bool   `json:"required"`
}

type CustomAgent struct {
	ID                  string               `json:"id"`
	Slug                string               `json:"slug"`
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	Role                string               `json:"role"`
	SystemPrompt        string               `json:"system_prompt"`
	Memory              string               `json:"memory"`
	ModelMode           string               `json:"model_mode"`
	ProviderID          string               `json:"provider_id"`
	ModelID             string               `json:"model_id"`
	ReasoningMode       string               `json:"reasoning_mode"`
	AutoModelHint       string               `json:"auto_model_hint"`
	AllowedTools        []string             `json:"allowed_tools"`
	WorkspacePath       string               `json:"workspace_path"`
	WorkflowID          string               `json:"workflow_id"`
	Enabled             bool                 `json:"enabled"`
	CreatedBy           string               `json:"created_by"`
	TriggerType         string               `json:"trigger_type"`
	IntervalMinutes     int                  `json:"interval_minutes"`
	CronExpr            string               `json:"cron_expr"`
	MonitorKind         string               `json:"monitor_kind"`
	MonitorTarget       string               `json:"monitor_target"`
	MonitorLastHash     string               `json:"monitor_last_hash"`
	MonitorLastSnapshot string               `json:"monitor_last_snapshot"`
	AcceptanceCriteria  string               `json:"acceptance_criteria"`
	DeterministicChecks []DeterministicCheck `json:"deterministic_checks"`
	VerifierAgentID     string               `json:"verifier_agent_id"`
	NextAgentID         string               `json:"next_agent_id"`
	MaxReworkAttempts   int                  `json:"max_rework_attempts"`
	StrictVerify        bool                 `json:"strict_verify"`
	Status              string               `json:"status"`
	LastError           string               `json:"last_error"`
	LastRunAt           int64                `json:"last_run_at"`
	NextRunAt           int64                `json:"next_run_at"`
	CreatedAt           int64                `json:"created_at"`
	UpdatedAt           int64                `json:"updated_at"`
}

type RunStep struct {
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

type CustomAgentRun struct {
	ID                  string    `json:"id"`
	AgentID             string    `json:"agent_id"`
	AgentName           string    `json:"agent_name"`
	AgentRole           string    `json:"agent_role"`
	ChainID             string    `json:"chain_id"`
	ParentRunID         string    `json:"parent_run_id"`
	TriggerReason       string    `json:"trigger_reason"`
	InputPrompt         string    `json:"input_prompt"`
	Status              string    `json:"status"`
	Attempt             int       `json:"attempt"`
	ResolvedProvider    string    `json:"resolved_provider"`
	ResolvedModel       string    `json:"resolved_model"`
	ResolvedReasoning   string    `json:"resolved_reasoning"`
	AutoModelReason     string    `json:"auto_model_reason"`
	OutputText          string    `json:"output_text"`
	VerifierVerdict     string    `json:"verifier_verdict"`
	VerifierFeedback    string    `json:"verifier_feedback"`
	DeterministicReport string    `json:"deterministic_report"`
	MonitorDiff         string    `json:"monitor_diff"`
	Steps               []RunStep `json:"steps"`
	Error               string    `json:"error"`
	StartedAt           int64     `json:"started_at"`
	FinishedAt          int64     `json:"finished_at"`
}

type BlackboardEntry struct {
	ID            string `json:"id"`
	ChainID       string `json:"chain_id"`
	RunID         string `json:"run_id"`
	FromAgentID   string `json:"from_agent_id"`
	FromAgentName string `json:"from_agent_name"`
	ToAgentID     string `json:"to_agent_id"`
	Topic         string `json:"topic"`
	Payload       string `json:"payload"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"created_at"`
}

type ModelOption struct {
	ProviderID           string   `json:"provider_id"`
	ProviderName         string   `json:"provider_name"`
	ModelID              string   `json:"model_id"`
	Name                 string   `json:"name"`
	Tier                 string   `json:"tier"`
	CanReason            bool     `json:"can_reason"`
	ReasoningModes       []string `json:"reasoning_modes"`
	ReasoningLevels      []string `json:"reasoning_levels"`
	DefaultReasoningMode string   `json:"default_reasoning_mode"`
	ContextWindow        int64    `json:"context_window"`
	CostPer1MIn          float64  `json:"cost_per_1m_in"`
	CostPer1MOut         float64  `json:"cost_per_1m_out"`
	Locked               bool     `json:"locked"`
	PlanLabel            string   `json:"plan_label"`
}

type ResolvedModelChoice struct {
	ProviderID    string `json:"provider_id"`
	ModelID       string `json:"model_id"`
	ReasoningMode string `json:"reasoning_mode"`
	Reason        string `json:"reason"`
	Tier          string `json:"tier"`
}

type ProbeResult struct {
	Changed  bool   `json:"changed"`
	Hash     string `json:"hash"`
	Snapshot string `json:"snapshot"`
	Diff     string `json:"diff"`
	Error    string `json:"error"`
}

type Event struct {
	Type       string          `json:"type"`
	AgentID    string          `json:"agent_id"`
	RunID      string          `json:"run_id"`
	ChainID    string          `json:"chain_id"`
	WorkflowID string          `json:"workflow_id,omitempty"`
	Workflow   *CustomWorkflow `json:"workflow,omitempty"`
	Payload    any             `json:"payload"`
	Timestamp  int64           `json:"timestamp"`
}

type ExecuteRequest struct {
	Agent             CustomAgent     `json:"agent"`
	ChainID           string          `json:"chain_id"`
	RunID             string          `json:"run_id"`
	Prompt            string          `json:"prompt"`
	MemorySnapshot    string          `json:"memory_snapshot"`
	BlackboardContext string          `json:"blackboard_context"`
	ProviderID        string          `json:"provider_id"`
	ModelID           string          `json:"model_id"`
	ReasoningMode     string          `json:"reasoning_mode"`
	OnStep            func(RunStep)   `json:"-"`
}

type ExecuteResult struct {
	OutputText string    `json:"output_text"`
	Steps      []RunStep `json:"steps"`
}

type DispatchRequest struct {
	AgentID       string `json:"agent_id"`
	Slug          string `json:"slug"`
	Prompt        string `json:"prompt"`
	TriggerReason string `json:"trigger_reason"`
	ChainID       string `json:"chain_id"`
	ParentRunID   string `json:"parent_run_id"`
	Force         bool   `json:"force"`
	Depth         int    `json:"depth"`
}

type AgentExecutor interface {
	Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error)
}

type ModelCatalogSource interface {
	ListAvailableModels(ctx context.Context) ([]ModelOption, error)
}

type EventSink interface {
	Publish(event Event)
}

type GlobalMemoryDoc struct {
	Memory      string `json:"memory"`
	UserProfile string `json:"user_profile"`
	Soul        string `json:"soul"`
	MemoryLimit int    `json:"memory_limit"`
	UserLimit   int    `json:"user_limit"`
	MemoryChars int    `json:"memory_chars"`
	UserChars   int    `json:"user_chars"`
}

type MemoryOp struct {
	Action  string `json:"action"`
	Content string `json:"content"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

type SwarmWorkerSpec struct {
	AgentID             string               `json:"agent_id"`
	Slug                string               `json:"slug"`
	Name                string               `json:"name"`
	Role                string               `json:"role"`
	Task                string               `json:"task"`
	Prompt              string               `json:"prompt"`
	SystemPrompt        string               `json:"system_prompt"`
	ModelMode           string               `json:"model_mode"`
	ProviderID          string               `json:"provider_id"`
	ModelID             string               `json:"model_id"`
	ReasoningMode       string               `json:"reasoning_mode"`
	AutoModelHint       string               `json:"auto_model_hint"`
	AllowedTools        []string             `json:"allowed_tools"`
	WorkspacePath       string               `json:"workspace_path"`
	AcceptanceCriteria  string               `json:"acceptance_criteria"`
	DeterministicChecks []DeterministicCheck `json:"deterministic_checks"`
	VerifierAgentID     string               `json:"verifier_agent_id"`
	MaxReworkAttempts   int                  `json:"max_rework_attempts"`
	StrictVerify        bool                 `json:"strict_verify"`
}

type SwarmRequest struct {
	Goal                string               `json:"goal"`
	ChainID             string               `json:"chain_id"`
	Workers             []SwarmWorkerSpec    `json:"workers"`
	AcceptanceCriteria  string               `json:"acceptance_criteria"`
	DeterministicChecks []DeterministicCheck `json:"deterministic_checks"`
	VerifierAgentID     string               `json:"verifier_agent_id"`
	SynthesizerAgentID  string               `json:"synthesizer_agent_id"`
	MaxReworkAttempts   int                  `json:"max_rework_attempts"`
	PersistWorkers      bool                 `json:"persist_workers"`
	SkipVerify          bool                 `json:"skip_verify"`
	SkipSynthesize      bool                 `json:"skip_synthesize"`
}

type SwarmWorkerResult struct {
	AgentID string         `json:"agent_id"`
	Slug    string         `json:"slug"`
	Name    string         `json:"name"`
	Task    string         `json:"task"`
	Run     CustomAgentRun `json:"run"`
	Output  string         `json:"output"`
	Status  string         `json:"status"`
	Error   string         `json:"error,omitempty"`
}

type SwarmResult struct {
	ChainID             string              `json:"chain_id"`
	Goal                string              `json:"goal"`
	Status              string              `json:"status"`
	Attempts            int                 `json:"attempts"`
	Workers             []SwarmWorkerResult `json:"workers"`
	VerifierVerdict     string              `json:"verifier_verdict"`
	VerifierFeedback    string              `json:"verifier_feedback"`
	DeterministicReport string              `json:"deterministic_report"`
	FinalOutput         string              `json:"final_output"`
	SynthesizerRun      *CustomAgentRun     `json:"synthesizer_run,omitempty"`
	Blackboard          []BlackboardEntry   `json:"blackboard,omitempty"`
	Error               string              `json:"error,omitempty"`
}

type CustomWorkflow struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
	CreatedBy    string `json:"created_by"`
	EntryAgentID string `json:"entry_agent_id"`
	LayoutJSON   string `json:"layout_json"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

type CreateWorkflowInput struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Enabled      *bool  `json:"enabled"`
	CreatedBy    string `json:"created_by"`
	EntryAgentID string `json:"entry_agent_id"`
	LayoutJSON   string `json:"layout_json"`
}

type UpdateWorkflowPatch struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	Enabled      *bool   `json:"enabled"`
	EntryAgentID *string `json:"entry_agent_id"`
	LayoutJSON   *string `json:"layout_json"`
}

type CreateAgentInput struct {
	ID                  string               `json:"id"`
	Slug                string               `json:"slug"`
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	Role                string               `json:"role"`
	SystemPrompt        string               `json:"system_prompt"`
	Memory              string               `json:"memory"`
	ModelMode           string               `json:"model_mode"`
	ProviderID          string               `json:"provider_id"`
	ModelID             string               `json:"model_id"`
	ReasoningMode       string               `json:"reasoning_mode"`
	AutoModelHint       string               `json:"auto_model_hint"`
	AllowedTools        []string             `json:"allowed_tools"`
	WorkspacePath       string               `json:"workspace_path"`
	WorkflowID          string               `json:"workflow_id"`
	Enabled             *bool                `json:"enabled"`
	CreatedBy           string               `json:"created_by"`
	TriggerType         string               `json:"trigger_type"`
	IntervalMinutes     int                  `json:"interval_minutes"`
	CronExpr            string               `json:"cron_expr"`
	MonitorKind         string               `json:"monitor_kind"`
	MonitorTarget       string               `json:"monitor_target"`
	AcceptanceCriteria  string               `json:"acceptance_criteria"`
	DeterministicChecks []DeterministicCheck `json:"deterministic_checks"`
	VerifierAgentID     string               `json:"verifier_agent_id"`
	NextAgentID         string               `json:"next_agent_id"`
	MaxReworkAttempts   int                  `json:"max_rework_attempts"`
	StrictVerify        bool                 `json:"strict_verify"`
}

type UpdateAgentPatch struct {
	Slug                *string               `json:"slug"`
	Name                *string               `json:"name"`
	Description         *string               `json:"description"`
	Role                *string               `json:"role"`
	SystemPrompt        *string               `json:"system_prompt"`
	Memory              *string               `json:"memory"`
	ModelMode           *string               `json:"model_mode"`
	ProviderID          *string               `json:"provider_id"`
	ModelID             *string               `json:"model_id"`
	ReasoningMode       *string               `json:"reasoning_mode"`
	AutoModelHint       *string               `json:"auto_model_hint"`
	AllowedTools        *[]string             `json:"allowed_tools"`
	WorkspacePath       *string               `json:"workspace_path"`
	WorkflowID          *string               `json:"workflow_id"`
	Enabled             *bool                 `json:"enabled"`
	TriggerType         *string               `json:"trigger_type"`
	IntervalMinutes     *int                  `json:"interval_minutes"`
	CronExpr            *string               `json:"cron_expr"`
	MonitorKind         *string               `json:"monitor_kind"`
	MonitorTarget       *string               `json:"monitor_target"`
	AcceptanceCriteria  *string               `json:"acceptance_criteria"`
	DeterministicChecks *[]DeterministicCheck `json:"deterministic_checks"`
	VerifierAgentID     *string               `json:"verifier_agent_id"`
	NextAgentID         *string               `json:"next_agent_id"`
	MaxReworkAttempts   *int                  `json:"max_rework_attempts"`
	StrictVerify        *bool                 `json:"strict_verify"`
	Status              *string               `json:"status"`
}

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/customagents"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	CustomAgentListToolName       = "custom_agent_list"
	CustomAgentListModelsToolName = "custom_agent_list_models"
	CustomAgentCreateToolName     = "custom_agent_create"
	CustomAgentUpdateToolName     = "custom_agent_update"
	CustomAgentDeleteToolName     = "custom_agent_delete"
	CustomAgentDispatchToolName   = "custom_agent_dispatch"
	CustomAgentSwarmToolName      = "custom_agent_swarm"
	CustomAgentRunsToolName       = "custom_agent_runs"
	CustomAgentMemoryToolName     = "custom_agent_memory"
	CustomAgentBlackboardToolName = "custom_agent_blackboard"
)

type customAgentSummary struct {
	ID              string `json:"id"`
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Role            string `json:"role"`
	Status          string `json:"status"`
	Enabled         bool   `json:"enabled"`
	ModelMode       string `json:"model_mode"`
	ProviderID      string `json:"provider_id,omitempty"`
	ModelID         string `json:"model_id,omitempty"`
	ReasoningMode   string `json:"reasoning_mode,omitempty"`
	AutoModelHint   string `json:"auto_model_hint,omitempty"`
	TriggerType     string `json:"trigger_type"`
	Description     string `json:"description,omitempty"`
	VerifierAgentID string `json:"verifier_agent_id,omitempty"`
	NextAgentID     string `json:"next_agent_id,omitempty"`
	StrictVerify    bool   `json:"strict_verify,omitempty"`
}

type CustomAgentListParams struct{}

type CustomAgentListModelsParams struct{}

type CustomAgentCreateParams struct {
	Name                string                            `json:"name" description:"Display name of the custom agent"`
	Slug                string                            `json:"slug,omitempty" description:"Optional unique slug"`
	Description         string                            `json:"description,omitempty" description:"Short description of the agent's responsibility"`
	Role                string                            `json:"role,omitempty" description:"Role: worker, orchestrator, or verifier"`
	SystemPrompt        string                            `json:"system_prompt" description:"System instructions for the agent"`
	ModelMode           string                            `json:"model_mode,omitempty" description:"Model selection mode: auto or manual"`
	ProviderID          string                            `json:"provider_id,omitempty" description:"Configured provider ID when model_mode is manual"`
	ModelID             string                            `json:"model_id,omitempty" description:"Configured model ID when model_mode is manual"`
	ReasoningMode       string                            `json:"reasoning_mode,omitempty" description:"Reasoning mode: off, low, medium, high, xhigh, or max"`
	AutoModelHint       string                            `json:"auto_model_hint,omitempty" description:"Optional tier hint for auto selection: fast, balanced, reasoning, or flagship"`
	AllowedTools        []string                          `json:"allowed_tools,omitempty" description:"Optional list of allowed standard tool names"`
	WorkspacePath       string                            `json:"workspace_path,omitempty" description:"Optional workspace directory path"`
	Enabled             *bool                             `json:"enabled,omitempty" description:"Whether the agent is enabled (defaults to true)"`
	TriggerType         string                            `json:"trigger_type,omitempty" description:"Trigger type: manual, interval, cron, event, or monitor"`
	IntervalMinutes     int                               `json:"interval_minutes,omitempty" description:"Interval in minutes for interval or monitor triggers"`
	CronExpr            string                            `json:"cron_expr,omitempty" description:"Cron expression for cron triggers"`
	MonitorKind         string                            `json:"monitor_kind,omitempty" description:"Monitor kind: command, http, or file"`
	MonitorTarget       string                            `json:"monitor_target,omitempty" description:"Command, URL, or file path to monitor"`
	AcceptanceCriteria  string                            `json:"acceptance_criteria,omitempty" description:"Acceptance criteria checked by verifier"`
	DeterministicChecks []customagents.DeterministicCheck `json:"deterministic_checks,omitempty" description:"Deterministic checks to run on output"`
	VerifierAgentID     string                            `json:"verifier_agent_id,omitempty" description:"Optional verifier agent ID"`
	NextAgentID         string                            `json:"next_agent_id,omitempty" description:"Optional next agent ID for chained handoff"`
	MaxReworkAttempts   int                               `json:"max_rework_attempts,omitempty" description:"Max rework attempts on verification failure"`
	StrictVerify        bool                              `json:"strict_verify,omitempty" description:"Require verifier pass even without explicit acceptance criteria"`
}

type CustomAgentUpdateParams struct {
	ID                  string                             `json:"id,omitempty" description:"Agent ID"`
	Slug                string                             `json:"slug,omitempty" description:"Agent slug used for lookup when id is omitted"`
	NewSlug             *string                            `json:"new_slug,omitempty" description:"Optional new slug"`
	Name                *string                            `json:"name,omitempty" description:"Updated display name"`
	Description         *string                            `json:"description,omitempty" description:"Updated description"`
	Role                *string                            `json:"role,omitempty" description:"Updated role: worker, orchestrator, or verifier"`
	SystemPrompt        *string                            `json:"system_prompt,omitempty" description:"Updated system prompt"`
	Memory              *string                            `json:"memory,omitempty" description:"Updated memory content"`
	ModelMode           *string                            `json:"model_mode,omitempty" description:"Updated model mode: auto or manual"`
	ProviderID          *string                            `json:"provider_id,omitempty" description:"Updated provider ID"`
	ModelID             *string                            `json:"model_id,omitempty" description:"Updated model ID"`
	ReasoningMode       *string                            `json:"reasoning_mode,omitempty" description:"Updated reasoning mode"`
	AutoModelHint       *string                            `json:"auto_model_hint,omitempty" description:"Updated auto model tier hint"`
	AllowedTools        *[]string                          `json:"allowed_tools,omitempty" description:"Updated allowed tools list"`
	WorkspacePath       *string                            `json:"workspace_path,omitempty" description:"Updated workspace path"`
	Enabled             *bool                              `json:"enabled,omitempty" description:"Enable or disable the agent"`
	TriggerType         *string                            `json:"trigger_type,omitempty" description:"Updated trigger type"`
	IntervalMinutes     *int                               `json:"interval_minutes,omitempty" description:"Updated interval minutes"`
	CronExpr            *string                            `json:"cron_expr,omitempty" description:"Updated cron expression"`
	MonitorKind         *string                            `json:"monitor_kind,omitempty" description:"Updated monitor kind"`
	MonitorTarget       *string                            `json:"monitor_target,omitempty" description:"Updated monitor target"`
	AcceptanceCriteria  *string                            `json:"acceptance_criteria,omitempty" description:"Updated acceptance criteria"`
	DeterministicChecks *[]customagents.DeterministicCheck `json:"deterministic_checks,omitempty" description:"Updated deterministic checks"`
	VerifierAgentID     *string                            `json:"verifier_agent_id,omitempty" description:"Updated verifier agent ID"`
	NextAgentID         *string                            `json:"next_agent_id,omitempty" description:"Updated next agent ID"`
	MaxReworkAttempts   *int                               `json:"max_rework_attempts,omitempty" description:"Updated max rework attempts"`
	StrictVerify        *bool                              `json:"strict_verify,omitempty" description:"Updated strict verification setting"`
	Status              *string                            `json:"status,omitempty" description:"Updated status"`
}

type CustomAgentDeleteParams struct {
	ID   string `json:"id,omitempty" description:"Agent ID to delete"`
	Slug string `json:"slug,omitempty" description:"Agent slug to delete if id is omitted"`
}

type CustomAgentDispatchParams struct {
	AgentID string `json:"agent_id,omitempty" description:"Target agent ID"`
	Slug    string `json:"slug,omitempty" description:"Target agent slug when agent_id is omitted"`
	Prompt  string `json:"prompt" description:"Task prompt to execute"`
	ChainID string `json:"chain_id,omitempty" description:"Optional chain ID to share blackboard context"`
	Wait    bool   `json:"wait,omitempty" description:"Wait synchronously for completion if true; dispatch in background if false"`
	Force   bool   `json:"force,omitempty" description:"Force execution even if monitor probe sees no change"`
}

type CustomAgentSwarmParams struct {
	Goal                string                            `json:"goal" description:"Overall swarm goal"`
	ChainID             string                            `json:"chain_id,omitempty" description:"Optional shared blackboard chain ID"`
	Workers             []customagents.SwarmWorkerSpec    `json:"workers" description:"List of parallel workers to execute"`
	AcceptanceCriteria  string                            `json:"acceptance_criteria,omitempty" description:"Acceptance criteria checked by verifier"`
	DeterministicChecks []customagents.DeterministicCheck `json:"deterministic_checks,omitempty" description:"Deterministic checks to run on aggregated output"`
	VerifierAgentID     string                            `json:"verifier_agent_id,omitempty" description:"Optional verifier agent ID"`
	SynthesizerAgentID  string                            `json:"synthesizer_agent_id,omitempty" description:"Optional synthesizer agent ID"`
	MaxReworkAttempts   int                               `json:"max_rework_attempts,omitempty" description:"Max rework attempts on verification failure"`
	PersistWorkers      bool                              `json:"persist_workers,omitempty" description:"Persist ad-hoc workers into the custom agents store"`
	SkipVerify          bool                              `json:"skip_verify,omitempty" description:"Skip verification stage"`
	SkipSynthesize      bool                              `json:"skip_synthesize,omitempty" description:"Skip synthesis stage"`
}

type CustomAgentRunsParams struct {
	AgentID string `json:"agent_id,omitempty" description:"Filter runs by agent ID"`
	Slug    string `json:"slug,omitempty" description:"Filter runs by agent slug"`
	Limit   int    `json:"limit,omitempty" description:"Maximum number of runs to return (default 20)"`
}

type CustomAgentMemoryParams struct {
	Target     string                  `json:"target,omitempty" description:"Memory target: agent (default), memory/global_memory, or user/user_profile"`
	Op         string                  `json:"op,omitempty" description:"Operation: read, add, patch, remove, replace, append (agent only), or batch"`
	AgentID    string                  `json:"agent_id,omitempty" description:"Target agent ID when target is agent"`
	Slug       string                  `json:"slug,omitempty" description:"Target agent slug when target is agent"`
	Content    string                  `json:"content,omitempty" description:"Memory content for add, replace, or append"`
	OldText    string                  `json:"old_text,omitempty" description:"Existing entry substring to match for patch or remove"`
	NewText    string                  `json:"new_text,omitempty" description:"Replacement text for patch"`
	Operations []customagents.MemoryOp `json:"operations,omitempty" description:"Batch of memory operations (add, patch, remove, replace)"`
}

type CustomAgentBlackboardParams struct {
	Op        string `json:"op" description:"Operation: post or read"`
	ChainID   string `json:"chain_id,omitempty" description:"Blackboard chain ID (defaults to current chain when inside a run)"`
	Topic     string `json:"topic,omitempty" description:"Message topic when posting"`
	Payload   string `json:"payload,omitempty" description:"Message content when posting"`
	ToAgentID string `json:"to_agent_id,omitempty" description:"Optional recipient agent ID when posting"`
	Limit     int    `json:"limit,omitempty" description:"Maximum entries to read (default 50)"`
}

type CustomAgentExecutionBinding struct {
	AgentID   string
	AgentName string
	ChainID   string
	RunID     string
}

func NewCustomAgentChatTools(svc *customagents.Service) []fantasy.AgentTool {
	if svc == nil {
		return nil
	}
	chatBinding := CustomAgentExecutionBinding{
		AgentID:   "main-chat",
		AgentName: "Prime",
	}
	return []fantasy.AgentTool{
		NewCustomAgentListTool(svc),
		NewCustomAgentListModelsTool(svc),
		NewCustomAgentCreateTool(svc),
		NewCustomAgentUpdateTool(svc),
		NewCustomAgentDeleteTool(svc),
		NewCustomAgentDispatchTool(svc),
		NewCustomAgentSwarmTool(svc),
		NewCustomAgentRunsTool(svc),
		NewCustomAgentMemoryTool(svc, ""),
		NewCustomAgentBlackboardTool(svc, chatBinding),
	}
}

func NewCustomAgentRoleTools(svc *customagents.Service, agent customagents.CustomAgent, chainID, runID string) []fantasy.AgentTool {
	if svc == nil {
		return nil
	}
	binding := CustomAgentExecutionBinding{
		AgentID:   agent.ID,
		AgentName: agent.Name,
		ChainID:   chainID,
		RunID:     runID,
	}
	tools := []fantasy.AgentTool{
		NewCustomAgentMemoryTool(svc, agent.ID),
		NewCustomAgentBlackboardTool(svc, binding),
	}
	switch agent.Role {
	case customagents.RoleOrchestrator:
		tools = append(tools,
			NewCustomAgentListTool(svc),
			NewCustomAgentListModelsTool(svc),
			NewCustomAgentCreateTool(svc),
			NewCustomAgentUpdateTool(svc),
			NewCustomAgentDispatchTool(svc),
			NewCustomAgentSwarmTool(svc),
			NewCustomAgentRunsTool(svc),
		)
	case customagents.RoleVerifier:
		tools = append(tools,
			NewCustomAgentListTool(svc),
			NewCustomAgentRunsTool(svc),
		)
	}
	return tools
}

func NewCustomAgentListTool(svc *customagents.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentListToolName,
		"List all persistent custom agents with their slug, role, status, model configuration, trigger, and description.",
		func(ctx context.Context, _ CustomAgentListParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil || svc.Store() == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			agents, err := svc.Store().ListAgents(ctx)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			out := make([]customAgentSummary, 0, len(agents))
			for _, a := range agents {
				out = append(out, customAgentSummary{
					ID:              a.ID,
					Slug:            a.Slug,
					Name:            a.Name,
					Role:            a.Role,
					Status:          a.Status,
					Enabled:         a.Enabled,
					ModelMode:       a.ModelMode,
					ProviderID:      a.ProviderID,
					ModelID:         a.ModelID,
					ReasoningMode:   a.ReasoningMode,
					AutoModelHint:   a.AutoModelHint,
					TriggerType:     a.TriggerType,
					Description:     a.Description,
					VerifierAgentID: a.VerifierAgentID,
					NextAgentID:     a.NextAgentID,
					StrictVerify:    a.StrictVerify,
				})
			}
			return jsonToolResponse(map[string]any{"agents": out})
		},
	)
}

func NewCustomAgentListModelsTool(svc *customagents.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentListModelsToolName,
		"List available configured providers and models with tier, reasoning capabilities, context window, cost, and lock status.",
		func(ctx context.Context, _ CustomAgentListModelsParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			models, err := svc.ListAvailableModels(ctx)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return jsonToolResponse(map[string]any{"models": models})
		},
	)
}

func NewCustomAgentCreateTool(svc *customagents.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentCreateToolName,
		"Create a new persistent custom agent with optional schedule, monitor, verification criteria, and model settings.",
		func(ctx context.Context, params CustomAgentCreateParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil || svc.Store() == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			name := strings.TrimSpace(params.Name)
			slug := strings.TrimSpace(params.Slug)
			if name == "" && slug == "" {
				return fantasy.NewTextErrorResponse("name or slug is required"), nil
			}
			if name == "" {
				name = slug
			}
			enabled := true
			if params.Enabled != nil {
				enabled = *params.Enabled
			}
			agent := customagents.CustomAgent{
				Slug:                slug,
				Name:                name,
				Description:         params.Description,
				Role:                params.Role,
				SystemPrompt:        params.SystemPrompt,
				ModelMode:           params.ModelMode,
				ProviderID:          params.ProviderID,
				ModelID:             params.ModelID,
				ReasoningMode:       params.ReasoningMode,
				AutoModelHint:       params.AutoModelHint,
				AllowedTools:        params.AllowedTools,
				WorkspacePath:       params.WorkspacePath,
				Enabled:             enabled,
				CreatedBy:           "ai",
				TriggerType:         params.TriggerType,
				IntervalMinutes:     params.IntervalMinutes,
				CronExpr:            params.CronExpr,
				MonitorKind:         params.MonitorKind,
				MonitorTarget:       params.MonitorTarget,
				AcceptanceCriteria:  params.AcceptanceCriteria,
				DeterministicChecks: params.DeterministicChecks,
				VerifierAgentID:     params.VerifierAgentID,
				NextAgentID:         params.NextAgentID,
				MaxReworkAttempts:   params.MaxReworkAttempts,
				StrictVerify:        params.StrictVerify,
			}
			created, err := svc.Store().CreateAgent(ctx, agent)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return jsonToolResponse(created)
		},
	)
}

func NewCustomAgentUpdateTool(svc *customagents.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentUpdateToolName,
		"Update an existing persistent custom agent by id or slug.",
		func(ctx context.Context, params CustomAgentUpdateParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil || svc.Store() == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			existing, err := resolveCustomAgent(ctx, svc.Store(), params.ID, params.Slug)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if params.NewSlug != nil {
				existing.Slug = *params.NewSlug
			}
			if params.Name != nil {
				existing.Name = *params.Name
			}
			if params.Description != nil {
				existing.Description = *params.Description
			}
			if params.Role != nil {
				existing.Role = *params.Role
			}
			if params.SystemPrompt != nil {
				existing.SystemPrompt = *params.SystemPrompt
			}
			if params.Memory != nil {
				existing.Memory = *params.Memory
			}
			if params.ModelMode != nil {
				existing.ModelMode = *params.ModelMode
			}
			if params.ProviderID != nil {
				existing.ProviderID = *params.ProviderID
			}
			if params.ModelID != nil {
				existing.ModelID = *params.ModelID
			}
			if params.ReasoningMode != nil {
				existing.ReasoningMode = *params.ReasoningMode
			}
			if params.AutoModelHint != nil {
				existing.AutoModelHint = *params.AutoModelHint
			}
			if params.AllowedTools != nil {
				existing.AllowedTools = *params.AllowedTools
			}
			if params.WorkspacePath != nil {
				existing.WorkspacePath = *params.WorkspacePath
			}
			if params.Enabled != nil {
				existing.Enabled = *params.Enabled
				if !existing.Enabled && existing.Status == customagents.StatusIdle {
					existing.Status = customagents.StatusPaused
				} else if existing.Enabled && existing.Status == customagents.StatusPaused {
					existing.Status = customagents.StatusIdle
				}
			}
			if params.TriggerType != nil {
				existing.TriggerType = *params.TriggerType
			}
			if params.IntervalMinutes != nil {
				existing.IntervalMinutes = *params.IntervalMinutes
			}
			if params.CronExpr != nil {
				existing.CronExpr = *params.CronExpr
			}
			if params.MonitorKind != nil {
				existing.MonitorKind = *params.MonitorKind
			}
			if params.MonitorTarget != nil {
				existing.MonitorTarget = *params.MonitorTarget
			}
			if params.AcceptanceCriteria != nil {
				existing.AcceptanceCriteria = *params.AcceptanceCriteria
			}
			if params.DeterministicChecks != nil {
				existing.DeterministicChecks = *params.DeterministicChecks
			}
			if params.VerifierAgentID != nil {
				existing.VerifierAgentID = *params.VerifierAgentID
			}
			if params.NextAgentID != nil {
				existing.NextAgentID = *params.NextAgentID
			}
			if params.MaxReworkAttempts != nil {
				existing.MaxReworkAttempts = *params.MaxReworkAttempts
			}
			if params.StrictVerify != nil {
				existing.StrictVerify = *params.StrictVerify
			}
			if params.Status != nil {
				existing.Status = *params.Status
			}
			updated, err := svc.Store().UpdateAgent(ctx, existing)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return jsonToolResponse(updated)
		},
	)
}

func NewCustomAgentDeleteTool(svc *customagents.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentDeleteToolName,
		"Delete a persistent custom agent by id or slug.",
		func(ctx context.Context, params CustomAgentDeleteParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil || svc.Store() == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			existing, err := resolveCustomAgent(ctx, svc.Store(), params.ID, params.Slug)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if err := svc.Store().DeleteAgent(ctx, existing.ID); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return jsonToolResponse(map[string]any{"deleted": true, "id": existing.ID, "slug": existing.Slug})
		},
	)
}

func NewCustomAgentDispatchTool(svc *customagents.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentDispatchToolName,
		"Dispatch a task to a custom agent by agent_id or slug, either synchronously (wait=true) or in the background (wait=false).",
		func(ctx context.Context, params CustomAgentDispatchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			req := customagents.DispatchRequest{
				AgentID:       strings.TrimSpace(params.AgentID),
				Slug:          strings.TrimSpace(params.Slug),
				Prompt:        params.Prompt,
				TriggerReason: customagents.TriggerManual,
				ChainID:       strings.TrimSpace(params.ChainID),
				Force:         params.Force,
			}
			var (
				run customagents.CustomAgentRun
				err error
			)
			if params.Wait {
				run, err = svc.DispatchSync(ctx, req)
			} else {
				run, err = svc.Dispatch(ctx, req)
			}
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return jsonToolResponse(run)
		},
	)
}

func NewCustomAgentSwarmTool(svc *customagents.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentSwarmToolName,
		"Dispatch a parallel multi-worker swarm with shared blackboard, verification, rework loop, and synthesis.",
		func(ctx context.Context, params CustomAgentSwarmParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			res, err := svc.DispatchSwarm(ctx, customagents.SwarmRequest{
				Goal:                params.Goal,
				ChainID:             params.ChainID,
				Workers:             params.Workers,
				AcceptanceCriteria:  params.AcceptanceCriteria,
				DeterministicChecks: params.DeterministicChecks,
				VerifierAgentID:     params.VerifierAgentID,
				SynthesizerAgentID:  params.SynthesizerAgentID,
				MaxReworkAttempts:   params.MaxReworkAttempts,
				PersistWorkers:      params.PersistWorkers,
				SkipVerify:          params.SkipVerify,
				SkipSynthesize:      params.SkipSynthesize,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return jsonToolResponse(res)
		},
	)
}

func NewCustomAgentRunsTool(svc *customagents.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentRunsToolName,
		"List recent runs for all custom agents or a specific agent by agent_id or slug.",
		func(ctx context.Context, params CustomAgentRunsParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil || svc.Store() == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			agentID := strings.TrimSpace(params.AgentID)
			if agentID == "" && strings.TrimSpace(params.Slug) != "" {
				agent, err := svc.Store().GetAgentBySlug(ctx, strings.TrimSpace(params.Slug))
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				agentID = agent.ID
			}
			limit := params.Limit
			if limit <= 0 {
				limit = 20
			}
			runs, err := svc.Store().ListRuns(ctx, agentID, limit)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return jsonToolResponse(map[string]any{"runs": runs})
		},
	)
}

func NewCustomAgentMemoryTool(svc *customagents.Service, boundAgentID string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentMemoryToolName,
		"Read or update persistent memory: per-agent memory (target=\"agent\"), shared global MEMORY.md (target=\"memory\"), or user profile USER.md (target=\"user\").",
		func(ctx context.Context, params CustomAgentMemoryParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil || svc.Store() == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			normTarget, err := normalizeMemoryToolTarget(params.Target)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if normTarget != "agent" {
				return runGlobalMemoryToolOp(ctx, svc, normTarget, params)
			}
			targetID := strings.TrimSpace(params.AgentID)
			if targetID == "" && strings.TrimSpace(params.Slug) != "" {
				agent, err := svc.Store().GetAgentBySlug(ctx, strings.TrimSpace(params.Slug))
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				targetID = agent.ID
			}
			if targetID == "" {
				targetID = strings.TrimSpace(boundAgentID)
			}
			if targetID == "" {
				return fantasy.NewTextErrorResponse("agent_id or slug is required"), nil
			}

			op := strings.ToLower(strings.TrimSpace(params.Op))
			if op == "" {
				op = "read"
			}
			switch op {
			case "read":
				mem, err := svc.Store().ReadMemory(ctx, targetID)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return jsonToolResponse(map[string]any{"agent_id": targetID, "memory": mem})
			case "replace":
				if err := svc.Store().WriteMemory(ctx, targetID, params.Content); err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return jsonToolResponse(map[string]any{"agent_id": targetID, "updated": true})
			case "append":
				if err := svc.Store().AppendMemory(ctx, targetID, params.Content); err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return jsonToolResponse(map[string]any{"agent_id": targetID, "appended": true})
			default:
				return fantasy.NewTextErrorResponse(fmt.Sprintf("unsupported op %q: use read, replace, or append", params.Op)), nil
			}
		},
	)
}

func normalizeMemoryToolTarget(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "agent":
		return "agent", nil
	case "memory", "global_memory", "memory.md":
		return "memory", nil
	case "user", "user_profile", "user.md":
		return "user", nil
	default:
		return "", fmt.Errorf("unsupported target %q: use agent, memory, or user", raw)
	}
}

func runGlobalMemoryToolOp(ctx context.Context, svc *customagents.Service, normTarget string, params CustomAgentMemoryParams) (fantasy.ToolResponse, error) {
	gm := svc.GlobalMemory()
	if gm == nil {
		return fantasy.NewTextErrorResponse("global memory unavailable"), nil
	}
	op := strings.ToLower(strings.TrimSpace(params.Op))
	if op == "" {
		if len(params.Operations) > 0 {
			op = "batch"
		} else {
			op = "read"
		}
	}
	switch op {
	case "read":
		doc, err := gm.GetDoc(ctx)
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		if normTarget == "user" {
			return jsonToolResponse(map[string]any{
				"target":  "user",
				"content": doc.UserProfile,
				"chars":   doc.UserChars,
				"limit":   doc.UserLimit,
			})
		}
		return jsonToolResponse(map[string]any{
			"target":  "memory",
			"content": doc.Memory,
			"chars":   doc.MemoryChars,
			"limit":   doc.MemoryLimit,
		})
	case "add", "patch", "remove", "replace", "batch":
		var ops []customagents.MemoryOp
		if op == "batch" {
			if len(params.Operations) == 0 {
				return fantasy.NewTextErrorResponse("operations is required for batch op"), nil
			}
			ops = params.Operations
		} else if len(params.Operations) > 0 {
			ops = params.Operations
		} else {
			ops = []customagents.MemoryOp{{
				Action:  op,
				Content: params.Content,
				OldText: params.OldText,
				NewText: params.NewText,
			}}
		}
		updated, err := gm.ApplyBatch(ctx, normTarget, ops)
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		doc, docErr := gm.GetDoc(ctx)
		if docErr != nil {
			return jsonToolResponse(map[string]any{
				"target":  normTarget,
				"updated": true,
				"content": updated,
			})
		}
		chars, limit := doc.MemoryChars, doc.MemoryLimit
		if normTarget == "user" {
			chars, limit = doc.UserChars, doc.UserLimit
		}
		return jsonToolResponse(map[string]any{
			"target":  normTarget,
			"updated": true,
			"content": updated,
			"chars":   chars,
			"limit":   limit,
		})
	default:
		return fantasy.NewTextErrorResponse(fmt.Sprintf("unsupported op %q for target %q: use read, add, patch, remove, replace, or batch", params.Op, normTarget)), nil
	}
}

func NewCustomAgentBlackboardTool(svc *customagents.Service, binding CustomAgentExecutionBinding) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CustomAgentBlackboardToolName,
		"Read or post shared coordination entries on the custom-agent chain blackboard.",
		func(ctx context.Context, params CustomAgentBlackboardParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil || svc.Store() == nil {
				return fantasy.NewTextErrorResponse("custom agents service unavailable"), nil
			}
			chainID := strings.TrimSpace(params.ChainID)
			if chainID == "" {
				chainID = strings.TrimSpace(binding.ChainID)
			}
			op := strings.ToLower(strings.TrimSpace(params.Op))
			switch op {
			case "post":
				if chainID == "" {
					return fantasy.NewTextErrorResponse("chain_id is required to post to blackboard"), nil
				}
				if strings.TrimSpace(params.Payload) == "" && strings.TrimSpace(params.Topic) == "" {
					return fantasy.NewTextErrorResponse("payload or topic is required"), nil
				}
				entry, err := svc.Store().PostBlackboard(ctx, customagents.BlackboardEntry{
					ChainID:       chainID,
					RunID:         binding.RunID,
					FromAgentID:   binding.AgentID,
					FromAgentName: binding.AgentName,
					ToAgentID:     strings.TrimSpace(params.ToAgentID),
					Topic:         strings.TrimSpace(params.Topic),
					Payload:       params.Payload,
					Status:        "open",
				})
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return jsonToolResponse(entry)
			case "read", "":
				limit := params.Limit
				if limit <= 0 {
					limit = 50
				}
				entries, err := svc.Store().ListBlackboard(ctx, chainID, limit)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return jsonToolResponse(map[string]any{"entries": entries})
			default:
				return fantasy.NewTextErrorResponse(fmt.Sprintf("unsupported op %q: use post or read", params.Op)), nil
			}
		},
	)
}

func resolveCustomAgent(ctx context.Context, store *customagents.Store, id, slug string) (customagents.CustomAgent, error) {
	id = strings.TrimSpace(id)
	slug = strings.TrimSpace(slug)
	if id != "" {
		agent, err := store.GetAgent(ctx, id)
		if err == nil {
			return agent, nil
		}
		if slug == "" {
			return customagents.CustomAgent{}, err
		}
	}
	if slug != "" {
		return store.GetAgentBySlug(ctx, slug)
	}
	return customagents.CustomAgent{}, errors.New("agent id or slug is required")
}

func jsonToolResponse(v any) (fantasy.ToolResponse, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	return fantasy.NewTextResponse(string(raw)), nil
}

const (
	hermesIdentityHeader    = "<hermes_identity>"
	hermesMemoryHeader      = "<hermes_memory>"
	customAgentsIndexHeader = "<custom_agents_index>"
)

func formatHermesRuntimeNote(soulSnapshot, memSnapshot string, agents []customagents.CustomAgent) string {
	var sections []string
	if trimmedSoul := strings.TrimSpace(soulSnapshot); trimmedSoul != "" {
		sections = append(sections, hermesIdentityHeader+"\n"+trimmedSoul+"\n</hermes_identity>")
	}
	if trimmedMem := strings.TrimSpace(memSnapshot); trimmedMem != "" {
		sections = append(sections, trimmedMem)
	}
	sections = append(sections, formatCustomAgentsIndex(agents))
	return strings.Join(sections, "\n\n")
}

func formatCustomAgentsIndex(agents []customagents.CustomAgent) string {
	var b strings.Builder
	b.WriteString(customAgentsIndexHeader)
	b.WriteByte('\n')
	activeCount := 0
	for _, a := range agents {
		if !a.Enabled {
			continue
		}
		activeCount++
		desc := strings.Join(strings.Fields(a.Description), " ")
		if len([]rune(desc)) > 120 {
			desc = string([]rune(desc)[:120]) + "..."
		}
		fmt.Fprintf(&b, "%s | %s | %s | %s | %s | %s\n", a.Slug, a.Name, a.Role, a.Status, a.ModelMode, desc)
	}
	if activeCount == 0 {
		b.WriteString("(no active custom agents yet)\n")
	}
	b.WriteString("Proactive coordination rules:\n")
	b.WriteString("1. For codebase engineering in the current repo, continue using the native code-team (explore, code, review via agent) and base coder/file/bash tools as before.\n")
	b.WriteString("2. For specialized tasks, ongoing monitoring, scheduled workflows, or multi-part autonomous work, proactively coordinate without waiting to be asked: dispatch matching active agents from <custom_agents_index> via custom_agent_dispatch, or inspect custom_agent_list_models, create specialized persistent agents (custom_agent_create with suitable model/reasoning or model_mode=\"auto\", strict_verify, acceptance_criteria), and launch custom_agent_swarm for parallel Blackboard + Verifier + Synthesis execution.\n")
	b.WriteString("3. Persist durable cross-session facts to custom_agent_memory(target=\"memory\") and user preferences to custom_agent_memory(target=\"user\").\n")
	b.WriteString("</custom_agents_index>")
	return b.String()
}

func withCustomAgentsNote(messages []fantasy.Message, note string) []fantasy.Message {
	if note == "" {
		return messages
	}
	for _, entry := range messages {
		if entry.Role != fantasy.MessageRoleSystem {
			continue
		}
		for _, part := range entry.Content {
			if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok &&
				(strings.HasPrefix(text.Text, hermesIdentityHeader) ||
					strings.HasPrefix(text.Text, hermesMemoryHeader) ||
					strings.HasPrefix(text.Text, customAgentsIndexHeader) ||
					strings.Contains(text.Text, customAgentsIndexHeader)) {
				return messages
			}
		}
	}
	return append([]fantasy.Message{fantasy.NewSystemMessage(note)}, messages...)
}

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/agent"
	"github.com/SpherePrime/CLI/internal/apigen"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/customagents"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
)

type customAgentsEventSink struct {
	broker *pubsub.Broker[customagents.Event]
}

func (s *customAgentsEventSink) Publish(ev customagents.Event) {
	if s == nil || s.broker == nil {
		return
	}
	s.broker.Publish(pubsub.UpdatedEvent, ev)
}

func (s *Server) CustomAgents() *customagents.Service {
	return s.customAgents
}

func (s *Server) initCustomAgents(ctx context.Context, cfg *config.ConfigStore) {
	if ctx == nil {
		ctx = context.Background()
	}
	store, err := customagents.OpenStore(customagents.DefaultDataDir())
	if err != nil {
		slog.Warn("Failed to open custom agents store", "error", err)
		return
	}
	if gm := store.GlobalMemory(); gm != nil {
		gm.SetUseConfigSoul(true)
	}
	if err := store.EnsureDefaults(ctx); err != nil {
		slog.Warn("Failed to seed default custom agents", "error", err)
	}
	swarmCtx, swarmCancel := context.WithCancel(ctx)
	broker := pubsub.NewBroker[customagents.Event]()
	exec := agent.NewCustomExecutor(cfg, agent.WithAccountService(s.account))
	svc := customagents.NewService(store, exec, exec, &customAgentsEventSink{broker: broker})
	exec.SetService(svc)
	svc.Start(ctx)

	s.customAgents = svc
	s.customAgentEvents = broker
	s.customAgentStore = store
	s.customAgentCtx = swarmCtx
	s.customAgentCancel = swarmCancel
	if s.backend != nil {
		s.backend.SetCustomAgents(svc)
	}
}

func (s *Server) stopCustomAgents() {
	if s.customAgentCancel != nil {
		s.customAgentCancel()
	}
	if s.customAgents != nil {
		s.customAgents.Stop()
	}
	if s.customAgentEvents != nil {
		s.customAgentEvents.Shutdown()
	}
	if s.customAgentStore != nil {
		_ = s.customAgentStore.Close()
	}
}

func (c *controllerV1) customAgentsService() *customagents.Service {
	if c.customAgents != nil {
		return c.customAgents
	}
	if c.server != nil {
		return c.server.customAgents
	}
	return nil
}

func (c *controllerV1) customAgentEventBroker() *pubsub.Broker[customagents.Event] {
	if c.customAgentEvents != nil {
		return c.customAgentEvents
	}
	if c.server != nil {
		return c.server.customAgentEvents
	}
	return nil
}

func (c *controllerV1) customAgentServerContext() context.Context {
	if c.server != nil && c.server.customAgentCtx != nil {
		return c.server.customAgentCtx
	}
	return context.Background()
}

type customAgentSwarmHTTPRequest struct {
	customagents.SwarmRequest
	Async bool `json:"async,omitempty"`
}

func (c *controllerV1) customAgentEndpoints() []apigen.Endpoint {
	return []apigen.Endpoint{
		apigen.Get("/v1/custom-agents").Summary("List custom agents").Tags("custom-agents").Fails(500, 503).Handle(c.handleGetCustomAgents),
		apigen.Post("/v1/custom-agents").Summary("Create custom agent").Tags("custom-agents").Status(201).Fails(400, 409, 500, 503).Handle(c.handlePostCustomAgent),
		apigen.Get("/v1/custom-agents/models").Summary("List available models for custom agents").Tags("custom-agents").Fails(500, 503).Handle(c.handleGetCustomAgentModels),
		apigen.Get("/v1/custom-agents/runs").Summary("List custom agent runs").Tags("custom-agents").QueryParam("agent_id", "Filter by agent ID").QueryParam("limit", "Maximum number of runs").Fails(500, 503).Handle(c.handleGetCustomAgentRuns),
		apigen.Get("/v1/custom-agents/blackboard").Summary("List custom agent blackboard entries").Tags("custom-agents").QueryParam("chain_id", "Filter by chain ID").QueryParam("limit", "Maximum number of entries").Fails(500, 503).Handle(c.handleGetCustomAgentBlackboard),
		apigen.Post("/v1/custom-agents/blackboard").Summary("Post custom agent blackboard entry").Tags("custom-agents").Status(201).Fails(400, 500, 503).Handle(c.handlePostCustomAgentBlackboard),
		apigen.Get("/v1/custom-agents/global-memory").Summary("Get custom agents global memory").Tags("custom-agents").Responds(customagents.GlobalMemoryDoc{}).Fails(500, 503).Handle(c.handleGetCustomAgentGlobalMemory),
		apigen.Put("/v1/custom-agents/global-memory").Summary("Save custom agents global memory").Tags("custom-agents").Accepts(customagents.GlobalMemoryDoc{}).Responds(customagents.GlobalMemoryDoc{}).Fails(400, 500, 503).Handle(c.handlePutCustomAgentGlobalMemory),
		apigen.Post("/v1/custom-agents/swarm").Summary("Dispatch custom agent swarm").Tags("custom-agents").Accepts(customAgentSwarmHTTPRequest{}).Responds(customagents.SwarmResult{}).Fails(400, 500, 503).Handle(c.handlePostCustomAgentSwarm),
		apigen.Get("/v1/custom-agents/workflows").Summary("List custom agent workflows").Tags("custom-agents").Fails(500, 503).Handle(c.handleGetCustomAgentWorkflows),
		apigen.Post("/v1/custom-agents/workflows").Summary("Create custom agent workflow").Tags("custom-agents").Accepts(customagents.CreateWorkflowInput{}).Responds(customagents.CustomWorkflow{}).Status(201).Fails(400, 409, 500, 503).Handle(c.handlePostCustomAgentWorkflow),
		apigen.Get("/v1/custom-agents/workflows/{workflow_id}/detail").Summary("Get custom agent workflow").Tags("custom-agents").PathParam("workflow_id", "Workflow ID or slug").Responds(customagents.CustomWorkflow{}).Fails(404, 500, 503).Handle(c.handleGetCustomAgentWorkflow),
		apigen.Patch("/v1/custom-agents/workflows/{workflow_id}").Summary("Update custom agent workflow").Tags("custom-agents").PathParam("workflow_id", "Workflow ID or slug").Accepts(customagents.UpdateWorkflowPatch{}).Responds(customagents.CustomWorkflow{}).Fails(400, 404, 409, 500, 503).Handle(c.handlePatchCustomAgentWorkflow),
		apigen.Delete("/v1/custom-agents/workflows/{workflow_id}").Summary("Delete custom agent workflow").Tags("custom-agents").PathParam("workflow_id", "Workflow ID or slug").QueryParam("delete_agents", "Delete agents belonging to the workflow").Fails(400, 404, 500, 503).Handle(c.handleDeleteCustomAgentWorkflow),
		apigen.Post("/v1/custom-agents/workflows/{workflow_id}/run").Summary("Run custom agent workflow").Tags("custom-agents").PathParam("workflow_id", "Workflow ID or slug").Responds(customagents.CustomAgentRun{}).Status(202).Fails(400, 404, 409, 500, 503).Handle(c.handlePostCustomAgentWorkflowRun),
		apigen.Get("/v1/custom-agents/events").Summary("Stream custom agent events (SSE)").Tags("custom-agents").SSE().Fails(503).Handle(c.handleGetCustomAgentEvents),
		apigen.Get("/v1/custom-agents/{agent_id}").Summary("Get custom agent").Tags("custom-agents").PathParam("agent_id", "Custom agent ID or slug").Fails(404, 500, 503).Handle(c.handleGetCustomAgent),
		apigen.Patch("/v1/custom-agents/{agent_id}").Summary("Update custom agent").Tags("custom-agents").PathParam("agent_id", "Custom agent ID or slug").Fails(400, 404, 409, 500, 503).Handle(c.handlePatchCustomAgent),
		apigen.Delete("/v1/custom-agents/{agent_id}").Summary("Delete custom agent").Tags("custom-agents").PathParam("agent_id", "Custom agent ID or slug").Fails(404, 500, 503).Handle(c.handleDeleteCustomAgent),
		apigen.Post("/v1/custom-agents/{agent_id}/run").Summary("Dispatch custom agent run").Tags("custom-agents").PathParam("agent_id", "Custom agent ID or slug").Status(202).Fails(400, 404, 409, 500, 503).Handle(c.handlePostCustomAgentRun),
		apigen.Post("/v1/custom-agents/{agent_id}/probe").Summary("Probe custom agent monitor").Tags("custom-agents").PathParam("agent_id", "Custom agent ID or slug").Fails(404, 500, 503).Handle(c.handlePostCustomAgentProbe),
		apigen.Get("/v1/custom-agents/{agent_id}/runs").Summary("List runs for a custom agent").Tags("custom-agents").PathParam("agent_id", "Custom agent ID or slug").QueryParam("limit", "Maximum number of runs").Fails(404, 500, 503).Handle(c.handleGetCustomAgentRunsByAgent),
		apigen.Post("/v1/custom-agents/{agent_id}/runs/{run_id}/cancel").Summary("Cancel active custom agent run").Tags("custom-agents").PathParam("agent_id", "Custom agent ID or slug").PathParam("run_id", "Run ID").Fails(404, 409, 500, 503).Handle(c.handlePostCustomAgentRunCancel),
	}
}

func (c *controllerV1) resolveCustomAgentParam(ctx context.Context, svc *customagents.Service, agentID string) (customagents.CustomAgent, error) {
	a, err := svc.Store().GetAgent(ctx, agentID)
	if err == nil {
		return a, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return svc.Store().GetAgentBySlug(ctx, agentID)
	}
	return customagents.CustomAgent{}, err
}

func (c *controllerV1) handleGetCustomAgents(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	agents, err := svc.Store().ListAgents(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if agents == nil {
		agents = []customagents.CustomAgent{}
	}
	jsonEncode(w, map[string]any{"agents": agents})
}

func (c *controllerV1) handlePostCustomAgent(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	var req struct {
		ID                  string                            `json:"id"`
		Slug                string                            `json:"slug"`
		Name                string                            `json:"name"`
		Description         string                            `json:"description"`
		Role                string                            `json:"role"`
		SystemPrompt        string                            `json:"system_prompt"`
		Memory              string                            `json:"memory"`
		ModelMode           string                            `json:"model_mode"`
		ProviderID          string                            `json:"provider_id"`
		ModelID             string                            `json:"model_id"`
		ReasoningMode       string                            `json:"reasoning_mode"`
		AutoModelHint       string                            `json:"auto_model_hint"`
		AllowedTools        []string                          `json:"allowed_tools"`
		WorkspacePath       string                            `json:"workspace_path"`
		WorkflowID          string                            `json:"workflow_id"`
		Enabled             *bool                             `json:"enabled"`
		CreatedBy           string                            `json:"created_by"`
		TriggerType         string                            `json:"trigger_type"`
		IntervalMinutes     int                               `json:"interval_minutes"`
		CronExpr            string                            `json:"cron_expr"`
		MonitorKind         string                            `json:"monitor_kind"`
		MonitorTarget       string                            `json:"monitor_target"`
		AcceptanceCriteria  string                            `json:"acceptance_criteria"`
		DeterministicChecks []customagents.DeterministicCheck `json:"deterministic_checks"`
		VerifierAgentID     string                            `json:"verifier_agent_id"`
		NextAgentID         string                            `json:"next_agent_id"`
		MaxReworkAttempts   int                               `json:"max_rework_attempts"`
		StrictVerify        bool                              `json:"strict_verify"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" && strings.TrimSpace(req.Slug) == "" {
		jsonError(w, http.StatusBadRequest, "name or slug is required")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = req.Slug
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	createdBy := strings.TrimSpace(req.CreatedBy)
	if createdBy == "" {
		createdBy = "user"
	}
	created, err := svc.Store().CreateAgent(r.Context(), customagents.CustomAgent{
		ID:                  strings.TrimSpace(req.ID),
		Slug:                strings.TrimSpace(req.Slug),
		Name:                strings.TrimSpace(req.Name),
		Description:         req.Description,
		Role:                req.Role,
		SystemPrompt:        req.SystemPrompt,
		Memory:              req.Memory,
		ModelMode:           req.ModelMode,
		ProviderID:          req.ProviderID,
		ModelID:             req.ModelID,
		ReasoningMode:       req.ReasoningMode,
		AutoModelHint:       req.AutoModelHint,
		AllowedTools:        req.AllowedTools,
		WorkspacePath:       req.WorkspacePath,
		WorkflowID:          resolveWorkflowIDForHTTP(r.Context(), svc, req.WorkflowID),
		Enabled:             enabled,
		CreatedBy:           createdBy,
		TriggerType:         req.TriggerType,
		IntervalMinutes:     req.IntervalMinutes,
		CronExpr:            req.CronExpr,
		MonitorKind:         req.MonitorKind,
		MonitorTarget:       req.MonitorTarget,
		AcceptanceCriteria:  req.AcceptanceCriteria,
		DeterministicChecks: req.DeterministicChecks,
		VerifierAgentID:     req.VerifierAgentID,
		NextAgentID:         req.NextAgentID,
		MaxReworkAttempts:   req.MaxReworkAttempts,
		StrictVerify:        req.StrictVerify,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			jsonError(w, http.StatusConflict, err.Error())
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncodeStatus(w, http.StatusCreated, created)
}

func (c *controllerV1) handleGetCustomAgentModels(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	models, err := svc.ListAvailableModels(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if models == nil {
		models = []customagents.ModelOption{}
	}
	jsonEncode(w, map[string]any{"models": models})
}

func (c *controllerV1) handleGetCustomAgentRuns(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	agentID := strings.TrimSpace(r.URL.Query().Get("agent_id"))
	limit := parsePositiveLimit(r.URL.Query().Get("limit"), 50)
	runs, err := svc.Store().ListRuns(r.Context(), agentID, limit)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runs == nil {
		runs = []customagents.CustomAgentRun{}
	}
	jsonEncode(w, map[string]any{"runs": runs})
}

func (c *controllerV1) handleGetCustomAgentBlackboard(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	chainID := strings.TrimSpace(r.URL.Query().Get("chain_id"))
	limit := parsePositiveLimit(r.URL.Query().Get("limit"), 50)
	entries, err := svc.Store().ListBlackboard(r.Context(), chainID, limit)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []customagents.BlackboardEntry{}
	}
	jsonEncode(w, map[string]any{"entries": entries})
}

func (c *controllerV1) handlePostCustomAgentBlackboard(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	var entry customagents.BlackboardEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(entry.ChainID) == "" {
		jsonError(w, http.StatusBadRequest, "chain_id is required")
		return
	}
	if strings.TrimSpace(entry.Payload) == "" && strings.TrimSpace(entry.Topic) == "" {
		jsonError(w, http.StatusBadRequest, "payload or topic is required")
		return
	}
	posted, err := svc.Store().PostBlackboard(r.Context(), entry)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncodeStatus(w, http.StatusCreated, posted)
}

func (c *controllerV1) handleGetCustomAgentEvents(w http.ResponseWriter, r *http.Request) {
	broker := c.customAgentEventBroker()
	if broker == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents events unavailable")
		return
	}
	flusher := http.NewResponseController(w)
	events := broker.Subscribe(r.Context())

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	_ = flusher.Flush()

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			_ = flusher.Flush()
		case ev, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(ev.Payload)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: custom_agent\ndata: %s\n\n", data)
			_ = flusher.Flush()
		}
	}
}

func (c *controllerV1) handleGetCustomAgent(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	agentObj, err := c.resolveCustomAgentParam(r.Context(), svc, r.PathValue("agent_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "custom agent not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncode(w, agentObj)
}

func (c *controllerV1) handlePatchCustomAgent(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	existing, err := c.resolveCustomAgentParam(r.Context(), svc, r.PathValue("agent_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "custom agent not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var patch struct {
		Slug                *string                            `json:"slug"`
		Name                *string                            `json:"name"`
		Description         *string                            `json:"description"`
		Role                *string                            `json:"role"`
		SystemPrompt        *string                            `json:"system_prompt"`
		Memory              *string                            `json:"memory"`
		ModelMode           *string                            `json:"model_mode"`
		ProviderID          *string                            `json:"provider_id"`
		ModelID             *string                            `json:"model_id"`
		ReasoningMode       *string                            `json:"reasoning_mode"`
		AutoModelHint       *string                            `json:"auto_model_hint"`
		AllowedTools        *[]string                          `json:"allowed_tools"`
		WorkspacePath       *string                            `json:"workspace_path"`
		WorkflowID          *string                            `json:"workflow_id"`
		Enabled             *bool                              `json:"enabled"`
		TriggerType         *string                            `json:"trigger_type"`
		IntervalMinutes     *int                               `json:"interval_minutes"`
		CronExpr            *string                            `json:"cron_expr"`
		MonitorKind         *string                            `json:"monitor_kind"`
		MonitorTarget       *string                            `json:"monitor_target"`
		AcceptanceCriteria  *string                            `json:"acceptance_criteria"`
		DeterministicChecks *[]customagents.DeterministicCheck `json:"deterministic_checks"`
		VerifierAgentID     *string                            `json:"verifier_agent_id"`
		NextAgentID         *string                            `json:"next_agent_id"`
		MaxReworkAttempts   *int                               `json:"max_rework_attempts"`
		StrictVerify        *bool                              `json:"strict_verify"`
		Status              *string                            `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if patch.Slug != nil {
		existing.Slug = *patch.Slug
	}
	if patch.Name != nil {
		existing.Name = *patch.Name
	}
	if patch.Description != nil {
		existing.Description = *patch.Description
	}
	if patch.Role != nil {
		existing.Role = *patch.Role
	}
	if patch.SystemPrompt != nil {
		existing.SystemPrompt = *patch.SystemPrompt
	}
	if patch.Memory != nil {
		existing.Memory = *patch.Memory
	}
	if patch.ModelMode != nil {
		existing.ModelMode = *patch.ModelMode
	}
	if patch.ProviderID != nil {
		existing.ProviderID = *patch.ProviderID
	}
	if patch.ModelID != nil {
		existing.ModelID = *patch.ModelID
	}
	if patch.ReasoningMode != nil {
		existing.ReasoningMode = *patch.ReasoningMode
	}
	if patch.AutoModelHint != nil {
		existing.AutoModelHint = *patch.AutoModelHint
	}
	if patch.AllowedTools != nil {
		existing.AllowedTools = *patch.AllowedTools
	}
	if patch.WorkspacePath != nil {
		existing.WorkspacePath = *patch.WorkspacePath
	}
	if patch.WorkflowID != nil {
		existing.WorkflowID = resolveWorkflowIDForHTTP(r.Context(), svc, *patch.WorkflowID)
	}
	if patch.Enabled != nil {
		existing.Enabled = *patch.Enabled
		if !existing.Enabled && existing.Status == customagents.StatusIdle {
			existing.Status = customagents.StatusPaused
		} else if existing.Enabled && existing.Status == customagents.StatusPaused {
			existing.Status = customagents.StatusIdle
		}
	}
	if patch.TriggerType != nil {
		existing.TriggerType = *patch.TriggerType
	}
	if patch.IntervalMinutes != nil {
		existing.IntervalMinutes = *patch.IntervalMinutes
	}
	if patch.CronExpr != nil {
		existing.CronExpr = *patch.CronExpr
	}
	if patch.MonitorKind != nil {
		existing.MonitorKind = *patch.MonitorKind
	}
	if patch.MonitorTarget != nil {
		existing.MonitorTarget = *patch.MonitorTarget
	}
	if patch.AcceptanceCriteria != nil {
		existing.AcceptanceCriteria = *patch.AcceptanceCriteria
	}
	if patch.DeterministicChecks != nil {
		existing.DeterministicChecks = *patch.DeterministicChecks
	}
	if patch.VerifierAgentID != nil {
		existing.VerifierAgentID = *patch.VerifierAgentID
	}
	if patch.NextAgentID != nil {
		existing.NextAgentID = *patch.NextAgentID
	}
	if patch.MaxReworkAttempts != nil {
		existing.MaxReworkAttempts = *patch.MaxReworkAttempts
	}
	if patch.StrictVerify != nil {
		existing.StrictVerify = *patch.StrictVerify
	}
	if patch.Status != nil {
		existing.Status = *patch.Status
	}

	updated, err := svc.Store().UpdateAgent(r.Context(), existing)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			jsonError(w, http.StatusConflict, err.Error())
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncode(w, updated)
}

func (c *controllerV1) handleDeleteCustomAgent(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	existing, err := c.resolveCustomAgentParam(r.Context(), svc, r.PathValue("agent_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "custom agent not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := svc.Store().DeleteAgent(r.Context(), existing.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "custom agent not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncode(w, map[string]bool{"deleted": true})
}

func (c *controllerV1) handlePostCustomAgentRun(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	existing, err := c.resolveCustomAgentParam(r.Context(), svc, r.PathValue("agent_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "custom agent not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req struct {
		Prompt        string `json:"prompt"`
		TriggerReason string `json:"trigger_reason"`
		ChainID       string `json:"chain_id"`
		Force         bool   `json:"force"`
		Sync          bool   `json:"sync"`
	}
	if r.Body != nil {
		if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil && !errors.Is(decErr, io.EOF) {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	dispatchReq := customagents.DispatchRequest{
		AgentID:       existing.ID,
		Prompt:        req.Prompt,
		TriggerReason: req.TriggerReason,
		ChainID:       req.ChainID,
		Force:         req.Force,
	}
	var run customagents.CustomAgentRun
	if req.Sync {
		run, err = svc.DispatchSync(r.Context(), dispatchReq)
	} else {
		run, err = svc.Dispatch(r.Context(), dispatchReq)
	}
	if err != nil {
		if strings.Contains(err.Error(), "уже выполняется") {
			jsonError(w, http.StatusConflict, err.Error())
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncodeStatus(w, http.StatusAccepted, run)
}

func (c *controllerV1) handlePostCustomAgentProbe(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	existing, err := c.resolveCustomAgentParam(r.Context(), svc, r.PathValue("agent_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "custom agent not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result, err := svc.ProbeAgent(r.Context(), existing.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncode(w, result)
}

func (c *controllerV1) handleGetCustomAgentRunsByAgent(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	existing, err := c.resolveCustomAgentParam(r.Context(), svc, r.PathValue("agent_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "custom agent not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	limit := parsePositiveLimit(r.URL.Query().Get("limit"), 50)
	runs, err := svc.Store().ListRuns(r.Context(), existing.ID, limit)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runs == nil {
		runs = []customagents.CustomAgentRun{}
	}
	jsonEncode(w, map[string]any{"runs": runs})
}

func (c *controllerV1) handlePostCustomAgentRunCancel(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	existing, err := c.resolveCustomAgentParam(r.Context(), svc, r.PathValue("agent_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "custom agent not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	runID := r.PathValue("run_id")
	run, err := svc.Store().GetRun(r.Context(), runID)
	if err != nil || run.AgentID != existing.ID {
		jsonError(w, http.StatusNotFound, "custom agent run not found")
		return
	}
	if err := svc.CancelRun(runID); err != nil {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}
	jsonEncode(w, map[string]bool{"cancelled": true})
}

func (c *controllerV1) handleGetCustomAgentGlobalMemory(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.GlobalMemory() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	doc, err := svc.GlobalMemory().GetDoc(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncode(w, doc)
}

func (c *controllerV1) handlePutCustomAgentGlobalMemory(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.GlobalMemory() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	var doc customagents.GlobalMemoryDoc
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	saved, err := svc.GlobalMemory().SaveDoc(r.Context(), doc)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonEncode(w, saved)
}

func (c *controllerV1) handlePostCustomAgentSwarm(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	var req customAgentSwarmHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Goal) == "" {
		jsonError(w, http.StatusBadRequest, "goal is required")
		return
	}
	if len(req.Workers) == 0 {
		jsonError(w, http.StatusBadRequest, "at least one worker is required")
		return
	}
	if req.Async {
		chainID := strings.TrimSpace(req.ChainID)
		if chainID == "" {
			chainID = "swarm-" + uuid.NewString()
		}
		req.SwarmRequest.ChainID = chainID
		bgCtx := c.customAgentServerContext()
		go func() {
			_, _ = svc.DispatchSwarm(bgCtx, req.SwarmRequest)
		}()
		jsonEncodeStatus(w, http.StatusAccepted, map[string]any{
			"chain_id": chainID,
			"status":   customagents.RunRunning,
		})
		return
	}
	res, err := svc.DispatchSwarm(r.Context(), req.SwarmRequest)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if res.Status == "" {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncode(w, res)
}

func resolveWorkflowIDForHTTP(ctx context.Context, svc *customagents.Service, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || svc == nil {
		return ""
	}
	if wf, err := svc.GetWorkflow(ctx, trimmed); err == nil && wf.ID != "" {
		return wf.ID
	}
	return trimmed
}

func (c *controllerV1) handleGetCustomAgentWorkflows(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	workflows, err := svc.ListWorkflows(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if workflows == nil {
		workflows = []customagents.CustomWorkflow{}
	}
	jsonEncode(w, map[string]any{"workflows": workflows})
}

func (c *controllerV1) handlePostCustomAgentWorkflow(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	var input customagents.CreateWorkflowInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(input.CreatedBy) == "" {
		input.CreatedBy = "user"
	}
	created, err := svc.CreateWorkflow(r.Context(), input)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "UNIQUE constraint"):
			jsonError(w, http.StatusConflict, err.Error())
		case strings.Contains(err.Error(), "workflow name or slug is required"),
			strings.Contains(err.Error(), "layout_json"):
			jsonError(w, http.StatusBadRequest, err.Error())
		default:
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonEncodeStatus(w, http.StatusCreated, created)
}

func (c *controllerV1) handleGetCustomAgentWorkflow(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	wf, err := svc.GetWorkflow(r.Context(), r.PathValue("workflow_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "custom workflow not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncode(w, wf)
}

func (c *controllerV1) handlePatchCustomAgentWorkflow(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	var patch customagents.UpdateWorkflowPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := svc.UpdateWorkflow(r.Context(), r.PathValue("workflow_id"), patch)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			jsonError(w, http.StatusNotFound, "custom workflow not found")
		case strings.Contains(err.Error(), "UNIQUE constraint"):
			jsonError(w, http.StatusConflict, err.Error())
		case strings.Contains(err.Error(), "workflow name cannot be empty"),
			strings.Contains(err.Error(), "layout_json"):
			jsonError(w, http.StatusBadRequest, err.Error())
		default:
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonEncode(w, updated)
}

func (c *controllerV1) handleDeleteCustomAgentWorkflow(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	deleteAgents := false
	if raw := strings.TrimSpace(r.URL.Query().Get("delete_agents")); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			jsonError(w, http.StatusBadRequest, "invalid delete_agents parameter")
			return
		}
		deleteAgents = parsed
	}
	if err := svc.DeleteWorkflow(r.Context(), r.PathValue("workflow_id"), deleteAgents); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			jsonError(w, http.StatusNotFound, "custom workflow not found")
		case strings.Contains(err.Error(), "cannot delete default workflow"):
			jsonError(w, http.StatusBadRequest, err.Error())
		default:
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonEncode(w, map[string]bool{"deleted": true})
}

func (c *controllerV1) handlePostCustomAgentWorkflowRun(w http.ResponseWriter, r *http.Request) {
	svc := c.customAgentsService()
	if svc == nil || svc.Store() == nil {
		jsonError(w, http.StatusServiceUnavailable, "custom agents service unavailable")
		return
	}
	var req struct {
		Prompt string `json:"prompt"`
	}
	if r.Body != nil {
		if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil && !errors.Is(decErr, io.EOF) {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	run, err := svc.RunWorkflow(r.Context(), r.PathValue("workflow_id"), req.Prompt)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			jsonError(w, http.StatusNotFound, "custom workflow not found")
		case strings.Contains(err.Error(), "уже выполняется"):
			jsonError(w, http.StatusConflict, err.Error())
		case strings.Contains(err.Error(), "отключён"),
			strings.Contains(err.Error(), "нет агентов"),
			strings.Contains(err.Error(), "нет активных агентов"):
			jsonError(w, http.StatusBadRequest, err.Error())
		default:
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonEncodeStatus(w, http.StatusAccepted, run)
}

func jsonEncodeStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parsePositiveLimit(raw string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

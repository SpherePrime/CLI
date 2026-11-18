package customagents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
)

func (s *Service) ListWorkflows(ctx context.Context) ([]CustomWorkflow, error) {
	if s.store == nil {
		return nil, errors.New("хранилище не настроено")
	}
	return s.store.ListWorkflows(ctx)
}

func (s *Service) GetWorkflow(ctx context.Context, idOrSlug string) (CustomWorkflow, error) {
	if s.store == nil {
		return CustomWorkflow{}, errors.New("хранилище не настроено")
	}
	return s.store.GetWorkflow(ctx, idOrSlug)
}

func (s *Service) CreateWorkflow(ctx context.Context, input CreateWorkflowInput) (CustomWorkflow, error) {
	if s.store == nil {
		return CustomWorkflow{}, errors.New("хранилище не настроено")
	}
	wf, err := s.store.CreateWorkflow(ctx, input)
	if err != nil {
		return CustomWorkflow{}, err
	}
	s.publish(Event{
		Type:       EventWorkflowUpdated,
		WorkflowID: wf.ID,
		Workflow:   &wf,
		Payload:    wf,
		Timestamp:  time.Now().Unix(),
	})
	return wf, nil
}

func (s *Service) UpdateWorkflow(ctx context.Context, id string, patch UpdateWorkflowPatch) (CustomWorkflow, error) {
	if s.store == nil {
		return CustomWorkflow{}, errors.New("хранилище не настроено")
	}
	wf, err := s.store.UpdateWorkflow(ctx, id, patch)
	if err != nil {
		return CustomWorkflow{}, err
	}
	s.publish(Event{
		Type:       EventWorkflowUpdated,
		WorkflowID: wf.ID,
		Workflow:   &wf,
		Payload:    wf,
		Timestamp:  time.Now().Unix(),
	})
	return wf, nil
}

func (s *Service) DeleteWorkflow(ctx context.Context, id string, deleteAgents bool) error {
	if s.store == nil {
		return errors.New("хранилище не настроено")
	}
	existing, err := s.store.GetWorkflow(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteWorkflow(ctx, existing.ID, deleteAgents); err != nil {
		return err
	}
	s.publish(Event{
		Type:       EventWorkflowDeleted,
		WorkflowID: existing.ID,
		Payload: map[string]any{
			"workflow_id":   existing.ID,
			"delete_agents": deleteAgents,
		},
		Timestamp: time.Now().Unix(),
	})
	return nil
}

func (s *Service) RunWorkflow(ctx context.Context, workflowID string, prompt string) (CustomAgentRun, error) {
	if s.store == nil {
		return CustomAgentRun{}, errors.New("хранилище не настроено")
	}
	wf, err := s.store.GetWorkflow(ctx, workflowID)
	if err != nil {
		return CustomAgentRun{}, err
	}
	if !wf.Enabled {
		return CustomAgentRun{}, fmt.Errorf("воркфлоу %s отключён", wf.Slug)
	}

	agents, err := s.store.ListAgentsByWorkflow(ctx, wf.ID)
	if err != nil {
		return CustomAgentRun{}, err
	}

	entry, err := selectWorkflowEntryAgent(wf, agents)
	if err != nil {
		return CustomAgentRun{}, err
	}

	return s.Dispatch(ctx, DispatchRequest{
		AgentID:       entry.ID,
		Prompt:        prompt,
		TriggerReason: "workflow:" + wf.Slug,
		ChainID:       "wf-" + wf.Slug + "-" + uuid.NewString()[:8],
		Force:         true,
	})
}

func selectWorkflowEntryAgent(wf CustomWorkflow, agents []CustomAgent) (CustomAgent, error) {
	if len(agents) == 0 {
		return CustomAgent{}, fmt.Errorf("в воркфлоу %s нет агентов для запуска", wf.Slug)
	}

	if target := strings.TrimSpace(wf.EntryAgentID); target != "" {
		for _, a := range agents {
			if a.ID == target || a.Slug == target {
				if !a.Enabled {
					return CustomAgent{}, fmt.Errorf("стартовый агент %s воркфлоу %s отключён", a.Slug, wf.Slug)
				}
				return a, nil
			}
		}
	}

	hasIncoming := make(map[string]bool, len(agents))
	for _, a := range agents {
		if a.NextAgentID != "" && a.NextAgentID != a.ID {
			hasIncoming[a.NextAgentID] = true
		}
	}

	for _, a := range agents {
		if a.Enabled && !hasIncoming[a.ID] && a.Role != RoleVerifier {
			return a, nil
		}
	}
	for _, a := range agents {
		if a.Enabled && !hasIncoming[a.ID] {
			return a, nil
		}
	}
	for _, a := range agents {
		if a.Enabled && a.Role != RoleVerifier {
			return a, nil
		}
	}
	for _, a := range agents {
		if a.Enabled {
			return a, nil
		}
	}

	return CustomAgent{}, fmt.Errorf("в воркфлоу %s нет активных агентов для запуска", wf.Slug)
}

func (s *Service) CreateAgent(ctx context.Context, input CreateAgentInput) (CustomAgent, error) {
	if s.store == nil {
		return CustomAgent{}, errors.New("хранилище не настроено")
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	createdBy := strings.TrimSpace(input.CreatedBy)
	if createdBy == "" {
		createdBy = "user"
	}
	agent := CustomAgent{
		ID:                  strings.TrimSpace(input.ID),
		Slug:                strings.TrimSpace(input.Slug),
		Name:                strings.TrimSpace(input.Name),
		Description:         input.Description,
		Role:                input.Role,
		SystemPrompt:        input.SystemPrompt,
		Memory:              input.Memory,
		ModelMode:           input.ModelMode,
		ProviderID:          input.ProviderID,
		ModelID:             input.ModelID,
		ReasoningMode:       input.ReasoningMode,
		AutoModelHint:       input.AutoModelHint,
		AllowedTools:        input.AllowedTools,
		WorkspacePath:       input.WorkspacePath,
		WorkflowID:          strings.TrimSpace(input.WorkflowID),
		Enabled:             enabled,
		CreatedBy:           createdBy,
		TriggerType:         input.TriggerType,
		IntervalMinutes:     input.IntervalMinutes,
		CronExpr:            input.CronExpr,
		MonitorKind:         input.MonitorKind,
		MonitorTarget:       input.MonitorTarget,
		AcceptanceCriteria:  input.AcceptanceCriteria,
		DeterministicChecks: input.DeterministicChecks,
		VerifierAgentID:     input.VerifierAgentID,
		NextAgentID:         input.NextAgentID,
		MaxReworkAttempts:   input.MaxReworkAttempts,
		StrictVerify:        input.StrictVerify,
	}
	created, err := s.store.CreateAgent(ctx, agent)
	if err != nil {
		return CustomAgent{}, err
	}
	s.publishStatus(created)
	return created, nil
}

func (s *Service) UpdateAgent(ctx context.Context, idOrSlug string, patch UpdateAgentPatch) (CustomAgent, error) {
	if s.store == nil {
		return CustomAgent{}, errors.New("хранилище не настроено")
	}
	existing, err := s.store.GetAgent(ctx, idOrSlug)
	if errors.Is(err, sql.ErrNoRows) {
		existing, err = s.store.GetAgentBySlug(ctx, idOrSlug)
	}
	if err != nil {
		return CustomAgent{}, err
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
		existing.WorkflowID = strings.TrimSpace(*patch.WorkflowID)
	}
	if patch.Enabled != nil {
		existing.Enabled = *patch.Enabled
		if !existing.Enabled && existing.Status == StatusIdle {
			existing.Status = StatusPaused
		} else if existing.Enabled && existing.Status == StatusPaused {
			existing.Status = StatusIdle
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

	updated, err := s.store.UpdateAgent(ctx, existing)
	if err != nil {
		return CustomAgent{}, err
	}
	s.publishStatus(updated)
	return updated, nil
}

func (s *Service) DeleteAgent(ctx context.Context, idOrSlug string) error {
	if s.store == nil {
		return errors.New("хранилище не настроено")
	}
	existing, err := s.store.GetAgent(ctx, idOrSlug)
	if errors.Is(err, sql.ErrNoRows) {
		existing, err = s.store.GetAgentBySlug(ctx, idOrSlug)
	}
	if err != nil {
		return err
	}
	return s.store.DeleteAgent(ctx, existing.ID)
}

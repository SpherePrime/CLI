package config

import (
	"fmt"
	"maps"
	"slices"
)

// OrchestratorConfig configures Prime's native Orchestrator mode.
type OrchestratorConfig struct {
	Enabled     bool           `json:"enabled,omitempty" jsonschema:"description=Enable Orchestrator mode by default for new chats"`
	Planner     *SelectedModel `json:"planner,omitempty" jsonschema:"description=Model and reasoning configuration for the read-only planner/reviewer"`
	Executor    *SelectedModel `json:"executor,omitempty" jsonschema:"description=Model and reasoning configuration for the fast executor"`
	FinalReview *bool          `json:"final_review,omitempty" jsonschema:"description=Run a final read-only verification pass by the planner after code edits"`
}

// OrchestratorUpdate describes a partial update to Orchestrator settings.
// Nil pointer fields leave the corresponding setting unchanged. Passing a
// SelectedModel with empty Provider and Model clears that role override.
type OrchestratorUpdate struct {
	Enabled     *bool          `json:"enabled,omitempty"`
	Planner     *SelectedModel `json:"planner,omitempty"`
	Executor    *SelectedModel `json:"executor,omitempty"`
	FinalReview *bool          `json:"final_review,omitempty"`
}

func cloneSelectedModelPtr(m *SelectedModel) *SelectedModel {
	if m == nil {
		return nil
	}
	cp := *m
	if m.Temperature != nil {
		v := *m.Temperature
		cp.Temperature = &v
	}
	if m.TopP != nil {
		v := *m.TopP
		cp.TopP = &v
	}
	if m.TopK != nil {
		v := *m.TopK
		cp.TopK = &v
	}
	if m.FrequencyPenalty != nil {
		v := *m.FrequencyPenalty
		cp.FrequencyPenalty = &v
	}
	if m.PresencePenalty != nil {
		v := *m.PresencePenalty
		cp.PresencePenalty = &v
	}
	if m.ProviderOptions != nil {
		cp.ProviderOptions = maps.Clone(m.ProviderOptions)
	}
	return &cp
}

func (o OrchestratorConfig) clone() OrchestratorConfig {
	cp := o
	cp.Planner = cloneSelectedModelPtr(o.Planner)
	cp.Executor = cloneSelectedModelPtr(o.Executor)
	if o.FinalReview != nil {
		v := *o.FinalReview
		cp.FinalReview = &v
	}
	return cp
}

// IsOrchestratorEnabled reports whether Orchestrator mode is active for
// sessionID. A per-session override in SessionOrchestrator takes precedence
// over Orchestrator.Enabled.
func (c *Config) IsOrchestratorEnabled(sessionID string) bool {
	if c == nil {
		return false
	}
	if sessionID != "" && c.SessionOrchestrator != nil {
		if enabled, ok := c.SessionOrchestrator[sessionID]; ok {
			return enabled
		}
	}
	return c.Orchestrator.Enabled
}

// OrchestratorFinalReviewEnabled reports whether the planner should run a final
// read-only verification pass after code edits. Defaults to true when unset.
func (c *Config) OrchestratorFinalReviewEnabled() bool {
	if c == nil || c.Orchestrator.FinalReview == nil {
		return true
	}
	return *c.Orchestrator.FinalReview
}

// ResolveOrchestratorPlanner returns the model configuration used by the
// Orchestrator planner/reviewer role.
func (c *Config) ResolveOrchestratorPlanner(sessionID string) SelectedModel {
	if c == nil {
		return SelectedModel{}
	}
	if c.Orchestrator.Planner != nil && c.Orchestrator.Planner.Provider != "" && c.Orchestrator.Planner.Model != "" {
		return *c.Orchestrator.Planner
	}
	if sessionID != "" && c.SessionModels != nil {
		if m, ok := c.SessionModels[sessionID]; ok && m.Provider != "" && m.Model != "" {
			return m
		}
	}
	if c.Models != nil {
		return c.Models[SelectedModelTypeLarge]
	}
	return SelectedModel{}
}

// ResolveOrchestratorExecutor returns the model configuration used by the
// Orchestrator executor role.
func (c *Config) ResolveOrchestratorExecutor() SelectedModel {
	if c == nil {
		return SelectedModel{}
	}
	if c.Orchestrator.Executor != nil && c.Orchestrator.Executor.Provider != "" && c.Orchestrator.Executor.Model != "" {
		return *c.Orchestrator.Executor
	}
	if c.Models != nil {
		if small, ok := c.Models[SelectedModelTypeSmall]; ok && small.Provider != "" && small.Model != "" {
			return small
		}
		return c.Models[SelectedModelTypeLarge]
	}
	return SelectedModel{}
}

// UpdateOrchestratorConfig updates Orchestrator settings in memory and persists
// them to models.json at the requested scope. Planner and Executor model
// selections are preserved when Enabled is toggled off.
func (s *ConfigStore) UpdateOrchestratorConfig(scope Scope, sessionID string, update OrchestratorUpdate) error {
	if update.Planner != nil && (update.Planner.Provider == "") != (update.Planner.Model == "") {
		return fmt.Errorf("planner model requires both provider and model")
	}
	if update.Executor != nil && (update.Executor.Provider == "") != (update.Executor.Model == "") {
		return fmt.Errorf("executor model requires both provider and model")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	var removeKeys []string
	err := s.updateLocked(scope, func(c *Config) map[string]any {
		fields := make(map[string]any)
		if update.Enabled != nil {
			if sessionID != "" {
				if c.SessionOrchestrator == nil {
					c.SessionOrchestrator = make(map[string]bool)
				}
				c.SessionOrchestrator[sessionID] = *update.Enabled
				fields["session_orchestrator"] = c.SessionOrchestrator
			} else {
				c.Orchestrator.Enabled = *update.Enabled
				fields["orchestrator.enabled"] = c.Orchestrator.Enabled
			}
		}
		if update.FinalReview != nil {
			val := *update.FinalReview
			c.Orchestrator.FinalReview = &val
			fields["orchestrator.final_review"] = val
		}
		if update.Planner != nil {
			if update.Planner.Provider == "" && update.Planner.Model == "" {
				c.Orchestrator.Planner = nil
				removeKeys = append(removeKeys, "orchestrator.planner")
			} else {
				sel := c.rememberModelSettings(*update.Planner)
				c.Orchestrator.Planner = &sel
				fields["orchestrator.planner"] = sel
				if len(c.ModelSettings) > 0 {
					fields["model_settings"] = c.ModelSettings
				}
			}
		}
		if update.Executor != nil {
			if update.Executor.Provider == "" && update.Executor.Model == "" {
				c.Orchestrator.Executor = nil
				removeKeys = append(removeKeys, "orchestrator.executor")
			} else {
				sel := c.rememberModelSettings(*update.Executor)
				c.Orchestrator.Executor = &sel
				fields["orchestrator.executor"] = sel
				if len(c.ModelSettings) > 0 {
					fields["model_settings"] = c.ModelSettings
				}
			}
		}
		return fields
	})
	if err != nil {
		return err
	}
	if len(removeKeys) > 0 {
		for _, key := range removeKeys {
			if err := s.removeConfigKey(scope, key, sectionFileFor(key)); err != nil {
				return err
			}
			if dir, dirErr := s.configDirForScope(scope); dirErr == nil {
				if legacy := legacyConfigPath(dir); fileHasContent(legacy) {
					if err := s.atomicWriteFileContents(legacy, deleteKeyFn(key)); err != nil {
						return err
					}
				}
			}
		}
		if dir, dirErr := s.configDirForScope(scope); dirErr == nil {
			s.captureStalenessSnapshot(append(slices.Clone(s.loadedPaths), configFilesForDir(dir)...))
		}
	}
	return nil
}

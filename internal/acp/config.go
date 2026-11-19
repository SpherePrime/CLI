package acp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

const (
	ModeCoder = "coder"
	ModePlan  = "plan"

	ConfigIDMode         = "mode"
	ConfigIDModel        = "model"
	ConfigIDThoughtLevel = "thought_level"
)

func buildSessionModes(currentMode string) *SessionModeState {
	if currentMode == "" {
		currentMode = ModeCoder
	}
	return &SessionModeState{
		CurrentModeID: currentMode,
		AvailableModes: []SessionMode{
			{
				ID:          ModeCoder,
				Name:        "Coder",
				Description: "Full coding agent with edit and execution tools",
			},
			{
				ID:          ModePlan,
				Name:        "Plan",
				Description: "Read-only planning and analysis mode",
			},
		},
	}
}

func (s *Server) sessionConfigOptions(sess *sessionState) []SessionConfigOption {
	cfg, _ := s.backend.GetWorkspaceConfig(sess.workspaceID)
	info, _ := s.backend.GetAgentInfo(sess.workspaceID)
	mode, reasoning := sess.getConfigState()
	return buildSessionConfigOptions(cfg, info, sess.id, mode, reasoning)
}

func buildSessionConfigOptions(
	cfg *config.Config,
	info proto.AgentInfo,
	sessionID string,
	currentMode string,
	currentReasoning string,
) []SessionConfigOption {
	if currentMode == "" {
		currentMode = ModeCoder
	}

	options := []SessionConfigOption{
		{
			ID:           ConfigIDMode,
			Name:         "Mode",
			Description:  "Agent operating mode",
			Category:     "mode",
			Type:         "select",
			CurrentValue: currentMode,
			Options: []SessionConfigSelectOption{
				{
					Value:       ModeCoder,
					Name:        "Coder",
					Description: "Full coding agent with edit and execution tools",
				},
				{
					Value:       ModePlan,
					Name:        "Plan",
					Description: "Read-only planning and analysis mode",
				},
			},
		},
	}

	selected, catwalkModel := resolveCurrentModel(cfg, info, sessionID)
	modelOpts := collectModelSelectOptions(cfg, selected)
	currentModelValue := formatModelValue(selected.Provider, selected.Model)
	if len(modelOpts) > 0 || currentModelValue != "" {
		options = append(options, SessionConfigOption{
			ID:           ConfigIDModel,
			Name:         "Model",
			Description:  "Language model for this session",
			Category:     "model",
			Type:         "select",
			CurrentValue: currentModelValue,
			Options:      modelOpts,
		})
	}

	if catwalkModel != nil && catwalkModel.CanReason {
		levelOpts := buildThoughtLevelOptions(catwalkModel.ReasoningLevels)
		effective := currentReasoning
		if effective != "" {
			effective = config.NormalizeReasoningMode(effective)
		} else {
			effective = selected.ResolveReasoningMode(*catwalkModel)
		}
		options = append(options, SessionConfigOption{
			ID:           ConfigIDThoughtLevel,
			Name:         "Thought Level",
			Description:  "Reasoning depth for thinking-capable models",
			Category:     "thought_level",
			Type:         "select",
			CurrentValue: effective,
			Options:      levelOpts,
		})
	}

	return options
}

func (s *Server) ApplyMode(sess *sessionState, modeID string) ([]SessionConfigOption, error) {
	var agentID string
	switch modeID {
	case ModeCoder:
		agentID = config.AgentGeneral
	case ModePlan:
		agentID = config.AgentPlan
	default:
		return nil, NewRPCError(CodeInvalidParams, fmt.Sprintf("invalid mode: %s", modeID))
	}

	if err := s.backend.SetMainAgent(sess.workspaceID, agentID); err != nil {
		if errors.Is(err, backend.ErrAgentBusy) {
			return nil, NewRPCError(CodeInvalidParams, err.Error())
		}
		return nil, NewRPCError(CodeInternalError, err.Error())
	}

	sess.setMode(modeID)
	options := s.sessionConfigOptions(sess)

	_ = s.sendSessionUpdate(sess.id, CurrentModeUpdate{
		SessionUpdate: "current_mode_update",
		CurrentModeID: modeID,
	})
	_ = s.sendSessionUpdate(sess.id, ConfigOptionUpdate{
		SessionUpdate: "config_option_update",
		ConfigOptions: options,
	})
	return options, nil
}

func (s *Server) ApplyModel(ctx context.Context, sess *sessionState, modelValue string) ([]SessionConfigOption, error) {
	trimmed := strings.TrimSpace(modelValue)
	if trimmed == "" {
		return nil, NewRPCError(CodeInvalidParams, "model value must not be empty")
	}

	cfg, err := s.backend.GetWorkspaceConfig(sess.workspaceID)
	if err != nil {
		return nil, NewRPCError(CodeInternalError, err.Error())
	}
	if cfg == nil {
		return nil, NewRPCError(CodeInvalidParams, fmt.Sprintf("unknown model: %s", modelValue))
	}

	providerID, modelID, catwalkModel := findConfiguredModel(cfg, trimmed)
	if catwalkModel == nil {
		return nil, NewRPCError(CodeInvalidParams, fmt.Sprintf("unknown model: %s", modelValue))
	}

	selected := config.SelectedModel{
		Provider:        providerID,
		Model:           modelID,
		MaxTokens:       catwalkModel.DefaultMaxTokens,
		ReasoningEffort: catwalkModel.DefaultReasoningEffort,
	}

	_, currentReasoning := sess.getConfigState()
	if catwalkModel.CanReason && currentReasoning != "" && isThoughtLevelSupported(catwalkModel, currentReasoning) {
		selected.ReasoningMode = currentReasoning
	} else {
		sess.setReasoningMode("")
	}

	if err := s.backend.UpdateSessionModel(sess.workspaceID, sess.id, &selected); err != nil {
		return nil, NewRPCError(CodeInvalidParams, err.Error())
	}
	if err := s.backend.UpdateAgent(ctx, sess.workspaceID); err != nil {
		return nil, NewRPCError(CodeInternalError, err.Error())
	}

	options := s.sessionConfigOptions(sess)
	_ = s.sendSessionUpdate(sess.id, ConfigOptionUpdate{
		SessionUpdate: "config_option_update",
		ConfigOptions: options,
	})
	return options, nil
}

func (s *Server) ApplyThoughtLevel(ctx context.Context, sess *sessionState, levelValue string) ([]SessionConfigOption, error) {
	cfg, err := s.backend.GetWorkspaceConfig(sess.workspaceID)
	if err != nil {
		return nil, NewRPCError(CodeInternalError, err.Error())
	}
	info, _ := s.backend.GetAgentInfo(sess.workspaceID)
	selected, catwalkModel := resolveCurrentModel(cfg, info, sess.id)
	if catwalkModel == nil || !catwalkModel.CanReason {
		return nil, NewRPCError(CodeInvalidParams, "current model does not support thought level")
	}

	normalized, err := config.NormalizeModelReasoningMode(*catwalkModel, levelValue)
	if err != nil {
		return nil, NewRPCError(CodeInvalidParams, err.Error())
	}
	if levelValue != config.ReasoningModeDefault && !isThoughtLevelSupported(catwalkModel, normalized) {
		return nil, NewRPCError(
			CodeInvalidParams,
			fmt.Sprintf("unsupported thought level %q for model %s", levelValue, catwalkModel.ID),
		)
	}

	selected.ReasoningMode = normalized
	if err := s.backend.UpdateSessionModel(sess.workspaceID, sess.id, &selected); err != nil {
		return nil, NewRPCError(CodeInvalidParams, err.Error())
	}
	if err := s.backend.UpdateAgent(ctx, sess.workspaceID); err != nil {
		return nil, NewRPCError(CodeInternalError, err.Error())
	}

	sess.setReasoningMode(normalized)
	options := s.sessionConfigOptions(sess)
	_ = s.sendSessionUpdate(sess.id, ConfigOptionUpdate{
		SessionUpdate: "config_option_update",
		ConfigOptions: options,
	})
	return options, nil
}

func resolveCurrentModel(cfg *config.Config, info proto.AgentInfo, sessionID string) (config.SelectedModel, *catwalk.Model) {
	var selected config.SelectedModel
	if cfg != nil && cfg.SessionModels != nil {
		if sm, ok := cfg.SessionModels[sessionID]; ok && sm.Provider != "" && sm.Model != "" {
			selected = sm
		}
	}
	if selected.Model == "" && info.ModelCfg.Provider != "" && info.ModelCfg.Model != "" {
		selected = info.ModelCfg
	}
	if selected.Model == "" && cfg != nil && cfg.Models != nil {
		if lm, ok := cfg.Models[config.SelectedModelTypeLarge]; ok {
			selected = lm
		}
	}

	var catwalkModel *catwalk.Model
	if cfg != nil && cfg.Providers != nil && selected.Provider != "" && selected.Model != "" {
		catwalkModel = cfg.GetModel(selected.Provider, selected.Model)
	}
	if catwalkModel == nil && info.Model.ID != "" {
		m := info.Model
		catwalkModel = &m
	}
	return selected, catwalkModel
}

func collectModelSelectOptions(cfg *config.Config, current config.SelectedModel) []SessionConfigSelectOption {
	if cfg == nil || cfg.Providers == nil {
		if current.Provider != "" && current.Model != "" {
			val := formatModelValue(current.Provider, current.Model)
			return []SessionConfigSelectOption{{Value: val, Name: val}}
		}
		return nil
	}

	providers := cfg.EnabledProviders()
	slices.SortFunc(providers, func(a, b config.ProviderConfig) int {
		return strings.Compare(a.ID, b.ID)
	})

	seen := make(map[string]struct{})
	var opts []SessionConfigSelectOption
	addModel := func(provider config.ProviderConfig, m catwalk.Model) {
		val := formatModelValue(provider.ID, m.ID)
		if _, exists := seen[val]; exists {
			return
		}
		seen[val] = struct{}{}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		providerName := provider.Name
		if providerName == "" {
			providerName = provider.ID
		}
		opts = append(opts, SessionConfigSelectOption{
			Value:       val,
			Name:        fmt.Sprintf("%s (%s)", name, providerName),
			Description: val,
		})
	}

	for _, provider := range providers {
		for _, m := range provider.Models {
			addModel(provider, m)
		}
		for _, m := range provider.ChatGPTModels {
			addModel(provider, m)
		}
	}

	if current.Provider != "" && current.Model != "" {
		val := formatModelValue(current.Provider, current.Model)
		if _, exists := seen[val]; !exists {
			opts = append([]SessionConfigSelectOption{{
				Value: val,
				Name:  val,
			}}, opts...)
		}
	}
	return opts
}

func findConfiguredModel(cfg *config.Config, value string) (string, string, *catwalk.Model) {
	if cfg == nil || cfg.Providers == nil {
		return "", "", nil
	}
	if providerID, modelID, hasSlash := strings.Cut(value, "/"); hasSlash && providerID != "" && modelID != "" {
		if providerCfg, ok := cfg.Providers.Get(providerID); ok && !providerCfg.Disable {
			if m := cfg.GetModel(providerID, modelID); m != nil {
				return providerID, modelID, m
			}
		}
	}

	providers := cfg.EnabledProviders()
	slices.SortFunc(providers, func(a, b config.ProviderConfig) int {
		return strings.Compare(a.ID, b.ID)
	})
	for _, provider := range providers {
		for i := range provider.Models {
			if provider.Models[i].ID == value {
				m := provider.Models[i]
				return provider.ID, m.ID, &m
			}
		}
		for i := range provider.ChatGPTModels {
			if provider.ChatGPTModels[i].ID == value {
				m := provider.ChatGPTModels[i]
				return provider.ID, m.ID, &m
			}
		}
	}
	return "", "", nil
}

func buildThoughtLevelOptions(levels []string) []SessionConfigSelectOption {
	seen := make(map[string]struct{})
	opts := make([]SessionConfigSelectOption, 0, len(levels)+1)
	add := func(mode string) {
		if _, ok := seen[mode]; ok {
			return
		}
		seen[mode] = struct{}{}
		opts = append(opts, SessionConfigSelectOption{
			Value: mode,
			Name:  config.FormatReasoningMode(mode),
		})
	}

	add(config.ReasoningModeOff)
	if len(levels) == 0 {
		for _, mode := range config.AllReasoningModes() {
			add(mode)
		}
		return opts
	}

	for _, raw := range levels {
		add(config.NormalizeReasoningMode(raw))
	}
	return opts
}

func isThoughtLevelSupported(model *catwalk.Model, normalized string) bool {
	if model == nil || !model.CanReason {
		return false
	}
	if normalized == "" || normalized == config.ReasoningModeOff {
		return true
	}
	if len(model.ReasoningLevels) == 0 {
		return slices.Contains(config.AllReasoningModes(), normalized)
	}
	for _, raw := range model.ReasoningLevels {
		if config.NormalizeReasoningMode(raw) == normalized {
			return true
		}
	}
	return false
}

func formatModelValue(providerID, modelID string) string {
	if providerID == "" || modelID == "" {
		return ""
	}
	return providerID + "/" + modelID
}

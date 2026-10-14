package model

import (
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

type visibleModeState struct {
	key         string
	enabled     bool
	detail      string
	unavailable bool
}

func thinkingModeState(model *workspace.AgentModel) visibleModeState {
	state := visibleModeState{key: "thinking"}
	if model == nil {
		state.unavailable = true
		return state
	}

	// Use the unified reasoning mode that honors both the new
	// ReasoningMode field and the legacy Think/ReasoningEffort fields.
	// This shows the current reasoning level (low/medium/high/xhigh/max)
	// in the status bar for any model, not just those marked CanReason=true.
	mode := model.ModelCfg.EffectiveReasoningMode()

	// If user hasn't explicitly set a mode, fall back to catalog defaults
	if mode == config.ReasoningModeOff && model.CatwalkCfg.CanReason && len(model.CatwalkCfg.ReasoningLevels) > 0 {
		// Check default effort
		defaultEffort := model.CatwalkCfg.DefaultReasoningEffort
		if defaultEffort != "" && slices.Contains(model.CatwalkCfg.ReasoningLevels, defaultEffort) {
			mode = config.NormalizeReasoningMode(defaultEffort)
		} else if len(model.CatwalkCfg.ReasoningLevels) > 0 {
			mode = config.NormalizeReasoningMode(model.CatwalkCfg.ReasoningLevels[0])
		}
	}

	if mode == config.ReasoningModeOff {
		state.enabled = false
		return state
	}

	state.enabled = true
	state.detail = config.FormatReasoningMode(mode)
	return state
}

func (s *Status) SetModel(model *workspace.AgentModel) {
	s.selectedModel = model
}

func (s *Status) renderModeStates(width int) string {
	if width <= 0 {
		return ""
	}
	states := []visibleModeState{
		thinkingModeState(s.selectedModel),
		{key: "plan", enabled: s.inputMode == uiInputModePlan},
		{key: "yolo", enabled: s.yolo},
	}
	var formats [3][]string
	for _, state := range states {
		value := s.com.L("status.mode.off")
		indicator := "-"
		color := s.com.Styles.Messages.AssistantInfoProvider.GetForeground()
		if state.enabled {
			value = s.com.L("status.mode.on")
			indicator = "+"
			color = s.com.Styles.Status.SuccessIndicator.GetForeground()
		}
		detail := state.detail
		if state.unavailable {
			detail = s.com.L("status.mode.unavailable")
		}
		full := s.com.L("status.mode."+state.key) + " " + value
		if detail != "" {
			full += "(" + detail + ")"
		}
		short := s.com.L("status.mode." + state.key + "_short")
		compact := short + ":" + value
		minimal := short + indicator
		if state.detail != "" {
			compact += "/" + state.detail
			minimal += state.detail
		}
		style := lipgloss.NewStyle().Foreground(color)
		formats[0] = append(formats[0], style.Render(full))
		formats[1] = append(formats[1], style.Render(compact))
		formats[2] = append(formats[2], style.Render(minimal))
	}
	for index, values := range formats {
		separator := " · "
		if index > 0 {
			separator = " "
		}
		rendered := strings.Join(values, separator)
		if ansi.StringWidth(rendered) <= width {
			return rendered
		}
	}
	return ansi.Truncate(strings.Join(formats[2], " "), width, "")
}

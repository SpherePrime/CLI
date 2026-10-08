package model

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/attachments"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestStatusShowsExplicitDisabledModes(t *testing.T) {
	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.status.helpKm = u
	u.status.SetMode(uiInputModeCode, false)
	badge := ansi.Strip(u.status.modeBadge())
	require.Contains(t, badge, "Thinking OFF")
	require.Contains(t, badge, "Plan OFF")
	require.Contains(t, badge, "YOLO OFF")
}

func TestThinkingModeReflectsEffectiveReasoning(t *testing.T) {
	tests := []struct {
		name        string
		model       *workspace.AgentModel
		enabled     bool
		detail      string
		unavailable bool
	}{
		{"missing model", nil, false, "", true},
		{"unsupported", &workspace.AgentModel{ModelCfg: config.SelectedModel{Think: true, ReasoningEffort: "high"}}, false, "", true},
		{"toggle off", &workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true}}, false, "", false},
		{"toggle on", &workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true}, ModelCfg: config.SelectedModel{Think: true}}, true, "", false},
		{"selected effort", &workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true, ReasoningLevels: []string{"medium", "high"}}, ModelCfg: config.SelectedModel{ReasoningEffort: "high"}}, true, "high", false},
		{"unsupported effort", &workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true}, ModelCfg: config.SelectedModel{ReasoningEffort: "medium"}}, false, "", false},
		{"default effort", &workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true, ReasoningLevels: []string{"medium", "high"}, DefaultReasoningEffort: "high"}}, true, "high", false},
		{"first effort", &workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true, ReasoningLevels: []string{"medium", "high"}}}, true, "medium", false},
		{"invalid selection", &workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true, ReasoningLevels: []string{"medium", "high"}, DefaultReasoningEffort: "medium"}, ModelCfg: config.SelectedModel{ReasoningEffort: "invalid"}}, true, "medium", false},
		{"none effort", &workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true, ReasoningLevels: []string{"none", "high"}}, ModelCfg: config.SelectedModel{ReasoningEffort: "none"}}, false, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var before workspace.AgentModel
			if test.model != nil {
				before = *test.model
			}
			state := thinkingModeState(test.model)
			require.Equal(t, test.enabled, state.enabled)
			require.Equal(t, test.detail, state.detail)
			require.Equal(t, test.unavailable, state.unavailable)
			if test.model != nil {
				require.Equal(t, before, *test.model)
			}
		})
	}
}

func TestModeStatesUpdateWithoutRestart(t *testing.T) {
	u := newPrismTestUI()
	selected := workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true}}
	u.status.SetModel(&selected)
	u.status.SetMode(uiInputModeCode, false)
	require.Contains(t, ansi.Strip(u.status.modeBadge()), "Thinking OFF")
	selected.ModelCfg.Think = true
	u.status.SetMode(uiInputModePlan, true)
	updated := ansi.Strip(u.status.modeBadge())
	require.Contains(t, updated, "Thinking ON")
	require.Contains(t, updated, "Plan ON")
	require.Contains(t, updated, "YOLO ON")
	selected.ModelCfg.Think = false
	selected.CatwalkCfg.ReasoningLevels = []string{"none", "high"}
	selected.ModelCfg.ReasoningEffort = "high"
	require.Contains(t, ansi.Strip(u.status.modeBadge()), "Thinking ON(high)")
	u.status.SetModel(nil)
	u.status.SetMode(uiInputModeCode, false)
	updated = ansi.Strip(u.status.modeBadge())
	require.Contains(t, updated, "Thinking OFF(unavailable)")
	require.Contains(t, updated, "Plan OFF")
	require.Contains(t, updated, "YOLO OFF")
}

func TestModeStatesDrawAcrossEveryDesignAndWidth(t *testing.T) {
	for _, design := range []string{"classic", "minimal", "cards", "dashboard", "terminal", "studio", "focus", "neon", "opencode", "paper", "blueprint", "ember"} {
		t.Run(design, func(t *testing.T) {
			u := newPrismTestUI()
			sty := styles.ResolveTheme("", "", design)
			u.com.Styles = &sty
			u.attachments = attachments.New(nil, attachments.Keymap{})
			u.status.helpKm = u
			selected := workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true, ReasoningLevels: []string{"high"}}, ModelCfg: config.SelectedModel{ReasoningEffort: "high"}}
			u.status.SetModel(&selected)
			u.status.SetMode(uiInputModePlan, true)
			for width := range 101 {
				require.LessOrEqual(t, ansi.StringWidth(u.status.modeBadgeWidth(width)), width)
			}
			line := strings.Join(drawStatusLines(t, u.status, 100, 1), "\n")
			require.Contains(t, line, "Thinking ON(high)")
			require.Contains(t, line, "Plan ON")
			require.Contains(t, line, "YOLO ON")
			selected.CatwalkCfg.ReasoningLevels = []string{"none", "high"}
			selected.ModelCfg.ReasoningEffort = "none"
			u.status.SetMode(uiInputModeCode, false)
			line = strings.Join(drawStatusLines(t, u.status, 100, 1), "\n")
			require.Contains(t, line, "Thinking OFF")
			require.Contains(t, line, "Plan OFF")
			require.Contains(t, line, "YOLO OFF")
		})
	}
}

type modeLocaleWorkspace struct {
	workspace.Workspace
	locale string
}

func (w modeLocaleWorkspace) Language() string { return w.locale }

func TestModeStatesTranslateAndPreserveVoice(t *testing.T) {
	sty := styles.ColorTonePantera()
	com := &common.Common{Workspace: modeLocaleWorkspace{locale: "ru"}, Styles: &sty}
	status := NewStatus(com, nil)
	selected := workspace.AgentModel{CatwalkCfg: catwalk.Model{CanReason: true}, ModelCfg: config.SelectedModel{Think: true}}
	status.SetModel(&selected)
	status.SetMode(uiInputModePlan, false)
	status.SetVoiceBadge("REC")
	badge := ansi.Strip(status.modeBadge())
	require.Contains(t, badge, "Размышления ВКЛ")
	require.Contains(t, badge, "План ВКЛ")
	require.Contains(t, badge, "YOLO ВЫКЛ")
	require.Contains(t, badge, "REC")
	for width := range 81 {
		require.LessOrEqual(t, ansi.StringWidth(status.modeBadgeWidth(width)), width)
	}
}

func TestStatusResourceSummaryPreservesModeStates(t *testing.T) {
	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.status.helpKm = u
	u.status.SetResourceSummary("MCP ON:1 OFF:2 · LSP WAIT:1 · Skills ON:3")
	u.status.SetMode(uiInputModePlan, false)
	lines := drawStatusLines(t, u.status, 100, 2)
	require.Contains(t, lines[0], "MCP ON:1 OFF:2")
	require.Contains(t, lines[0], "LSP WAIT:1")
	require.Contains(t, lines[0], "Skills ON:3")
	require.Contains(t, lines[1], "Thinking OFF")
	require.Contains(t, lines[1], "Plan ON")
	require.Contains(t, lines[1], "YOLO OFF")
	u.status.SetHideHelp(true)
	require.Empty(t, strings.TrimSpace(strings.Join(drawStatusLines(t, u.status, 100, 2), "")))
}

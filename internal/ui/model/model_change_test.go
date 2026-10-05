package model

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	"github.com/SpherePrime/CLI/internal/ui/util"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// Picking a model for the session has to change what the session runs on.
//
// The symptom this guards is the worst kind: the dialog closes, the toast says
// the model changed, and nothing else does - so the user cannot tell whether
// it worked. Both halves are asserted here, the config the UI reads and the
// write that reaches the store, because a change that only landed in one of
// them looks identical from the outside.
func TestSelectingAModelChangesWhatTheSessionRunsOn(t *testing.T) {
	u, ws := newAgentFlowUI(t)
	slot := config.SelectedModelTypeLarge

	require.Equal(t, "fast", ws.cfg.Models[slot].Model, "the test starts on a different model")

	cmd := u.handleSelectModel(dialog.ActionSelectModel{
		Provider:  catwalk.Provider{ID: catwalk.InferenceProvider("acme")},
		Model:     config.SelectedModel{Provider: "acme", Model: "slow"},
		ModelType: slot,
	})
	require.NotNil(t, cmd, "a successful selection must hand back the rebuild command")

	require.Equal(t, "slow", ws.cfg.Models[slot].Model,
		"the config the UI reads must hold the newly selected model")
	require.Contains(t, ws.written, "preferred_model",
		"the selection must reach the store, not just the in-memory config")
	require.False(t, u.dialog.ContainsDialog(dialog.ModelsID),
		"the picker closes once a model is chosen")
}

// Choosing while the picker is on the small slot must not write to the large
// one. The slot travels in the action, so a dialog that lost it would silently
// overwrite whichever model the user was not changing.
func TestSelectingAModelWritesToTheSlotItWasOpenedFor(t *testing.T) {
	u, ws := newAgentFlowUI(t)

	cmd := u.handleSelectModel(dialog.ActionSelectModel{
		Provider:  catwalk.Provider{ID: catwalk.InferenceProvider("acme")},
		Model:     config.SelectedModel{Provider: "acme", Model: "slow"},
		ModelType: config.SelectedModelTypeSmall,
	})
	require.NotNil(t, cmd)

	require.Equal(t, "slow", ws.cfg.Models[config.SelectedModelTypeSmall].Model,
		"the small slot must be the one that changed")
	require.Equal(t, "fast", ws.cfg.Models[config.SelectedModelTypeLarge].Model,
		"the large slot must be left alone")
}

// A selection the store rejects must not be reported as a change. Reporting
// success over a failed write is what makes the bug invisible to the person
// sitting at the terminal.
func TestAFailedWriteIsNotReportedAsASuccessfulChange(t *testing.T) {
	u, ws := newAgentFlowUI(t)
	ws.failPreferredModel = true

	cmd := u.handleSelectModel(dialog.ActionSelectModel{
		Provider:  catwalk.Provider{ID: catwalk.InferenceProvider("acme")},
		Model:     config.SelectedModel{Provider: "acme", Model: "slow"},
		ModelType: config.SelectedModelTypeLarge,
	})
	require.NotNil(t, cmd, "the failure must still be reported to the user")

	require.Equal(t, "fast", ws.cfg.Models[config.SelectedModelTypeLarge].Model,
		"a write that failed must leave the config alone")

	// The message the user sees has to be the error. The success toast is
	// built by the same command that rebuilds the agent, and it used to be
	// appended no matter what the write did, so a rejected change still told
	// the user the model had changed - the exact shape of a bug that cannot
	// be told apart from a working one while sitting at the terminal.
	var (
		reported  bool
		claimedIt bool
	)
	for _, msg := range collectCmdMessages(cmd) {
		info, ok := msg.(util.InfoMsg)
		if !ok {
			continue
		}
		if info.Type == util.InfoTypeError {
			reported = true
		}
		if strings.Contains(info.Msg, "changed to") {
			claimedIt = true
		}
	}
	require.True(t, reported, "a failed write must be reported as an error to the user")
	require.False(t, claimedIt,
		"nothing changed, so nothing may tell the user that it did")
}

// collectCmdMessages runs a command tree the way the runtime would and returns
// the messages its leaves produced, in order. runCmds drops messages it has no
// interest in, which is right for driving the UI and wrong for asserting on
// what the user was told.
func collectCmdMessages(cmd tea.Cmd) []tea.Msg {
	var out []tea.Msg
	var walk func(tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil {
			return
		}
		msg := c()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, next := range batch {
				walk(next)
			}
			return
		}
		if seq, ok := sequenceMsgOf(msg); ok {
			for _, next := range seq {
				walk(next)
			}
			return
		}
		out = append(out, msg)
	}
	walk(cmd)
	return out
}

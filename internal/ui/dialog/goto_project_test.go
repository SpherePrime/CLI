package dialog

import (
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/common"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// pressKey builds a plain key event the way the other dialog tests do:
// HandleMsg switches on msg.String(), so a named key needs its code rune
// and a text key needs its text.
func pressKey(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
	}
}

// newTestProjects builds a Projects dialog against the shared test theme.
// NewProjects reads the tracked-projects file through the global config
// path, which tests leave untouched; an empty list is fine for the modes
// these tests exercise.
func newTestProjects(t *testing.T) *Projects {
	t.Helper()
	com := &common.Common{Styles: testDialogStyles()}
	p, err := NewProjects(com)
	require.NoError(t, err)
	return p
}

func TestProjectsPathModeSwitchesPlaceholder(t *testing.T) {
	t.Parallel()

	p := newTestProjects(t)
	require.Equal(t, "Type to filter", p.input.Placeholder)

	p.HandleMsg(pressKey("ctrl+o"))
	require.Equal(t, projectsModePath, p.mode)
	require.Equal(t, "Enter or paste a folder path...", p.input.Placeholder)

	p.HandleMsg(pressKey("esc"))
	require.Equal(t, projectsModeNormal, p.mode)
}

func TestProjectsMissingPathAsksForConfirmation(t *testing.T) {
	t.Parallel()

	p := newTestProjects(t)
	p.mode = projectsModePath
	p.input.SetValue("Z:/definitely/not/here")

	action := p.HandleMsg(pressKey("enter"))
	require.Equal(t, projectsModeConfirm, p.mode, "a missing folder must ask before opening")
	require.Nil(t, action, "no switch may happen before the user confirms")

	// Declining returns to the path prompt with the typed path kept.
	p.HandleMsg(pressKey("n"))
	require.Equal(t, projectsModePath, p.mode)

	// Confirming switches to the typed path, normalized by the dialog to
	// an absolute, cleaned form.
	p.mode = projectsModeConfirm
	action = p.HandleMsg(pressKey("y"))
	require.IsType(t, ActionSwitchProject{}, action)
	require.Equal(t, filepath.Clean("Z:/definitely/not/here"), action.(ActionSwitchProject).Path)
}

func TestGoToProjectSubmitsTypedPath(t *testing.T) {
	t.Parallel()

	g := NewGoToProject(&common.Common{Styles: testDialogStyles()})
	g.input.SetValue("C:/some/project")

	action := g.HandleMsg(pressKey("enter"))
	require.IsType(t, ActionSwitchProject{}, action)
	require.Equal(t, "C:/some/project", action.(ActionSwitchProject).Path)
}

func TestGoToProjectEmptyInputDoesNothing(t *testing.T) {
	t.Parallel()

	g := NewGoToProject(&common.Common{Styles: testDialogStyles()})
	require.Nil(t, g.HandleMsg(pressKey("enter")), "an empty path must not switch")
}

func TestGoToProjectEscCloses(t *testing.T) {
	t.Parallel()

	g := NewGoToProject(&common.Common{Styles: testDialogStyles()})
	require.Equal(t, ActionClose{}, g.HandleMsg(pressKey("esc")))
}

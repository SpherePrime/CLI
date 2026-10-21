package agent

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type failedTitleModel struct {
	fastModel
	onStream func()
	success  bool
}

func (m failedTitleModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if m.onStream != nil {
		m.onStream()
	}
	if m.success {
		return m.fastModel.Stream(ctx, call)
	}
	return nil, context.Canceled
}

func TestGenerateTitleFailureUsesPrompt(t *testing.T) {
	for _, test := range []struct {
		name   string
		prompt string
		want   string
	}{
		{"russian", "  Исправь\n\tназвания чатов  ", "Исправь названия чатов"},
		{"long", strings.Repeat("я", 60), strings.Repeat("я", 49) + "…"},
		{"context", "Почини поиск\n\n<working-context>\nTask goal:\nДругая задача\n</working-context>", "Почини поиск"},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := testEnv(t)
			created, err := env.sessions.Create(t.Context(), DefaultSessionName)
			require.NoError(t, err)
			model := failedTitleModel{}
			agent := testSessionAgent(env, model, model, "system")
			agent.GenerateTitle(t.Context(), created.ID, test.prompt)
			updated, err := env.sessions.Get(t.Context(), created.ID)
			require.NoError(t, err)
			require.Equal(t, test.want, updated.Title)
			require.True(t, utf8.ValidString(updated.Title))
		})
	}
}

func TestGenerateTitlePreservesManualRename(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(map[bool]string{false: "failure", true: "success"}[success], func(t *testing.T) {
			testGenerateTitlePreservesManualRename(t, success)
		})
	}
}

func testGenerateTitlePreservesManualRename(t *testing.T, success bool) {
	env := testEnv(t)
	created, err := env.sessions.Create(t.Context(), DefaultSessionName)
	require.NoError(t, err)
	model := failedTitleModel{success: success, onStream: func() {
		require.NoError(t, env.sessions.Rename(t.Context(), created.ID, "Моё название"))
	}}
	agent := testSessionAgent(env, model, model, "system")
	agent.GenerateTitle(t.Context(), created.ID, "Исправь названия чатов")
	updated, err := env.sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "Моё название", updated.Title)
}

func TestGenerateTitlePreservesExistingTitle(t *testing.T) {
	env := testEnv(t)
	created, err := env.sessions.Create(t.Context(), "Моё название")
	require.NoError(t, err)
	agent := testSessionAgent(env, fastModel{}, fastModel{}, "system")
	agent.GenerateTitle(t.Context(), created.ID, "Исправь названия чатов")
	updated, err := env.sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "Моё название", updated.Title)
}

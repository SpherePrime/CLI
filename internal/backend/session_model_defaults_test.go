package backend

import (
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestUpdateSessionModelBecomesNewChatDefault(t *testing.T) {
	b, ws, _ := newPublishingWorkspace(t)
	selected := config.SelectedModel{Provider: "openai", Model: "gpt-4", ContextWindow: 250000}
	require.NoError(t, b.UpdateSessionModel(ws.ID, "first-chat", &selected))
	require.Equal(t, selected, ws.Cfg.Config().SessionModels["first-chat"])
	require.Equal(t, selected, ws.Cfg.Config().Models[config.SelectedModelTypeLarge])
	next := config.SelectedModel{Provider: "openai", Model: "gpt-4-next", ContextWindow: 128000}
	require.NoError(t, b.UpdateSessionModel(ws.ID, "first-chat", &next))
	require.Equal(t, next, ws.Cfg.Config().SessionModels["first-chat"])
	require.Equal(t, next, ws.Cfg.Config().Models[config.SelectedModelTypeLarge])
	selected = next
	require.NoError(t, b.UpdateSessionModel(ws.ID, "first-chat", nil))
	require.Equal(t, selected, ws.Cfg.Config().Models[config.SelectedModelTypeLarge])
}

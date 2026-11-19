package chat

import (
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/i18n"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestAssistantInfoOrchestratorPhaseBadge(t *testing.T) {
	t.Parallel()

	sty := styles.ApplyDesign(styles.ColorTonePantera(), "classic")
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("anthropic", config.ProviderConfig{
		ID:     "anthropic",
		Name:   "Anthropic",
		Models: []catwalk.Model{{ID: "claude-sonnet-4", Name: "Claude Sonnet 4"}},
	})

	t.Run("shows English phase badges on intermediate and final turns", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Providers: providers,
			Options:   &config.Options{Language: i18n.En},
		}

		planMsg := &message.Message{
			ID:       "m-plan",
			Role:     message.Assistant,
			Provider: "anthropic",
			Model:    "claude-sonnet-4",
			Parts: []message.ContentPart{
				message.OrchestratorStepMeta{Phase: "plan"},
				message.Finish{Reason: message.FinishReasonToolUse, Time: 10},
			},
		}
		require.True(t, ShouldShowAssistantInfo(planMsg))
		planItem := NewAssistantInfoItem(&sty, planMsg, cfg, time.Unix(0, 0)).(*AssistantInfoItem)
		require.Contains(t, ansi.Strip(planItem.renderContent(80)), "[Planner] Claude Sonnet 4")

		reviewMsg := &message.Message{
			ID:       "m-review",
			Role:     message.Assistant,
			Provider: "anthropic",
			Model:    "claude-sonnet-4",
			Parts: []message.ContentPart{
				message.OrchestratorStepMeta{Phase: "review"},
				message.Finish{Reason: message.FinishReasonEndTurn, Time: 10},
			},
		}
		require.True(t, ShouldShowAssistantInfo(reviewMsg))
		reviewItem := NewAssistantInfoItem(&sty, reviewMsg, cfg, time.Unix(0, 0)).(*AssistantInfoItem)
		require.Contains(t, ansi.Strip(reviewItem.renderContent(80)), "[Review] Claude Sonnet 4")
	})

	t.Run("shows Russian phase badges when language is ru", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Providers: providers,
			Options:   &config.Options{Language: i18n.Ru},
		}

		execMsg := &message.Message{
			ID:       "m-exec",
			Role:     message.Assistant,
			Provider: "anthropic",
			Model:    "claude-sonnet-4",
			Parts: []message.ContentPart{
				message.OrchestratorStepMeta{Phase: "execute"},
				message.Finish{Reason: message.FinishReasonToolUse, Time: 10},
			},
		}
		item := NewAssistantInfoItem(&sty, execMsg, cfg, time.Unix(0, 0)).(*AssistantInfoItem)
		require.Contains(t, ansi.Strip(item.renderContent(80)), "[Исполнитель] Claude Sonnet 4")
	})
}

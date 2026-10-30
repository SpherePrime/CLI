package agent

import (
	"context"
	"errors"

	"github.com/SpherePrime/CLI/internal/config"
)

// autoContinuePrompt resumes a run that stopped on a provider error. It is
// English because it is model-facing, like every tool description.
const autoContinuePrompt = "Continue the task from where it stopped. The previous attempt stopped on a provider error; keep going until the task is fully complete."

// autoContinueEnabled reports whether a run that stopped with err should be
// resumed automatically. A cancelled run and a run that finished on its own
// are never continued, and options.auto_continue_attempts bounds a
// permanently failing run instead of looping forever.
func autoContinueEnabled(cfg *config.Config, ctx context.Context, err error, attempt int) bool {
	if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}
	if cfg == nil || cfg.Options == nil || !cfg.Options.AutoContinue {
		return false
	}
	return attempt < cfg.Options.GetAutoContinueAttempts()
}

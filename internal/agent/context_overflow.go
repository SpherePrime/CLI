package agent

import (
	"errors"
	"fmt"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const contextOverflowContinuationPrompt = "The previous request exceeded the model context window, so the conversation was summarized. Continue the original request: `%s`"

func isContextOverflowError(err error) bool {
	var providerErr *fantasy.ProviderError
	return errors.As(err, &providerErr) && providerErr.IsContextTooLarge()
}

func contextOverflowContinuation(prompt string) string {
	return fmt.Sprintf(contextOverflowContinuationPrompt, prompt)
}

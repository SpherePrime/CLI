package openai

import (
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

var contextOverflowCodes = []string{
	"context_length_exceeded",
	"context_window_exceeded",
	"exceed_context_size_error",
	"prompt_too_long",
}

var contextOverflowPhrases = []string{
	"maximum context length",
	"context length exceeded",
	"context_length_exceeded",
	"exceeds the context window",
	"exceeds the available context",
	"exceeds the context size",
	"context window exceeded",
	"context size has been exceeded",
	"is loaded with context length",
	"prompt is too long",
	"input is too long",
	"longer than the model's context",
}

func markContextOverflow(code, errType, message string, providerErr *fantasy.ProviderError) {
	if providerErr.IsContextTooLarge() {
		return
	}
	for _, value := range []string{code, errType} {
		value = strings.ToLower(strings.TrimSpace(value))
		for _, overflowCode := range contextOverflowCodes {
			if value == overflowCode {
				providerErr.ContextTooLargeErr = true
				return
			}
		}
	}
	lowerMessage := strings.ToLower(message)
	for _, phrase := range contextOverflowPhrases {
		if strings.Contains(lowerMessage, phrase) {
			providerErr.ContextTooLargeErr = true
			return
		}
	}
}

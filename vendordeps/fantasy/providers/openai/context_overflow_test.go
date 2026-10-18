package openai

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestMarkContextOverflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    string
		errType string
		message string
		want    bool
	}{
		{name: "responses api code", code: "context_length_exceeded", message: "Your input exceeds the context window of this model.", want: true},
		{name: "llama.cpp", errType: "exceed_context_size_error", message: "the request exceeds the available context size", want: true},
		{name: "vllm", message: "This model's maximum context length is 32768 tokens. However, you requested 40000 tokens.", want: true},
		{name: "lm studio", message: "The model is loaded with context length of only 4096 tokens", want: true},
		{name: "rate limit", code: "rate_limit_exceeded", message: "Rate limit reached for requests"},
		{name: "invalid key", code: "invalid_api_key", message: "Incorrect API key provided"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			providerErr := &fantasy.ProviderError{}
			markContextOverflow(tc.code, tc.errType, tc.message, providerErr)
			require.Equal(t, tc.want, providerErr.IsContextTooLarge())
		})
	}
}

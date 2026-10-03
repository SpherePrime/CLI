package agent

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// Title generation tries a small model, then a large one. That fallback is
// only worth its cost for failures that belong to a single model.
func TestTitleFallbackWorthwhile(t *testing.T) {
	// Provider- and account-wide failures. The second model is behind the same
	// key, so it would repeat the failure after four more requests.
	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusPaymentRequired,
		http.StatusTooManyRequests,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			require.False(t, titleFallbackWorthwhile(&fantasy.ProviderError{StatusCode: status}))
		})
	}

	// An oversized prompt fails the same way on every model.
	require.False(t, titleFallbackWorthwhile(&fantasy.ProviderError{ContextTooLargeErr: true}))

	// Model-specific failures are exactly what the fallback exists for.
	require.True(t, titleFallbackWorthwhile(&fantasy.ProviderError{
		StatusCode: http.StatusNotFound,
		Message:    "The model `x` does not exist",
	}))
	require.True(t, titleFallbackWorthwhile(&fantasy.ProviderError{
		StatusCode: http.StatusBadRequest,
		Message:    "This model is currently unavailable",
	}))

	// Not a provider error: a different model gets a fresh connection.
	require.True(t, titleFallbackWorthwhile(errors.New("connection reset by peer")))
	require.True(t, titleFallbackWorthwhile(nil))

	// A provider error stays recognizable through wrapping.
	wrapped := fmt.Errorf("generate title: %w", &fantasy.ProviderError{StatusCode: http.StatusTooManyRequests})
	require.False(t, titleFallbackWorthwhile(wrapped))
}

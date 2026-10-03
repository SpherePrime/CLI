package fantasy

import (
	"io"
	"net/http"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// A mid-stream error event rides inside a 200 response, so it has no status
// code and TransientError is the only signal IsRetryable() can read. These
// cases pin the deny-list policy: known-permanent types stay permanent.
func TestIsTransientStreamErrorPermanent(t *testing.T) {
	for _, errType := range []string{
		"invalid_request_error",
		"invalid_api_key",
		"authentication_error",
		"permission_error",
		"permission_denied",
		"account_error",
		"billing_error",
		"insufficient_quota",
		"not_found_error",
		"model_not_found",
		"context_length_exceeded",
		"prompt_too_long",
		"content_policy_violation",
		"safety",
		"  Invalid_Request_Error\t", // casing and padding must not matter
	} {
		t.Run(errType, func(t *testing.T) {
			require.False(t, IsTransientStreamError(errType))
		})
	}
}

// Everything else retries. The old allow-list dropped each of these: an
// unrecognized type meant "permanent", so a provider asking the caller to come
// back failed on the first attempt with no retry at all.
func TestIsTransientStreamErrorRetriesUnrecognized(t *testing.T) {
	for _, errType := range []string{
		"server_error",
		"internal_error",
		"overloaded_error",
		"api_error",
		"rate_limit_error",
		"rate_limit_exceeded", // OpenAI's older code string, never in the allow-list
		"model_unavailable",
		"service_unavailable",
		"engine_overloaded",
		"throttling_exception",
		"resource_exhausted",
		"unknown_future_type",
		"", // provider sent no classification at all
	} {
		t.Run(errType, func(t *testing.T) {
			require.True(t, IsTransientStreamError(errType))
		})
	}
}

func TestIsRetryableMidStreamError(t *testing.T) {
	permanent := &ProviderError{Message: "This model is currently unavailable"}
	permanent.TransientError = IsTransientStreamError("invalid_request_error")
	require.False(t, permanent.IsRetryable(),
		"a mid-stream invalid request must not retry: resending changes nothing")

	unavailable := &ProviderError{Message: "This model is currently unavailable"}
	unavailable.TransientError = IsTransientStreamError("model_unavailable")
	require.True(t, unavailable.IsRetryable(),
		"an unavailable model is temporary; the retry loop has to see that")
}

// Request-level failures keep their HTTP status, so classification must not
// override it: a 400 stays permanent even under the retry-by-default policy.
func TestIsRetryableStatusCodeDrives(t *testing.T) {
	require.True(t, (&ProviderError{StatusCode: http.StatusTooManyRequests}).IsRetryable())
	require.True(t, (&ProviderError{StatusCode: http.StatusRequestTimeout}).IsRetryable())
	require.True(t, (&ProviderError{StatusCode: http.StatusConflict}).IsRetryable())
	require.True(t, (&ProviderError{StatusCode: 529}).IsRetryable(), "Anthropic overload")
	require.False(t, (&ProviderError{StatusCode: http.StatusBadRequest}).IsRetryable())
	require.False(t, (&ProviderError{StatusCode: http.StatusUnauthorized}).IsRetryable())
}

func TestIsRetryableWithoutStatusCode(t *testing.T) {
	require.False(t, (&ProviderError{}).IsRetryable(), "no signal at all")
	require.True(t, (&ProviderError{Cause: io.ErrUnexpectedEOF}).IsRetryable())
	require.True(t, NewIncompleteStreamError().IsRetryable())
	require.True(t, (&ProviderError{ResponseHeaders: map[string]string{"x-should-retry": "true"}}).IsRetryable())
	require.False(t, (&ProviderError{ResponseHeaders: map[string]string{"x-should-retry": "false"}}).IsRetryable())
}

func TestPermanentStreamErrorTypesHaveNoDuplicates(t *testing.T) {
	// A duplicate key would silently compile away one clause of the policy.
	require.Len(t, PermanentStreamErrorTypes, len(permanentStreamErrorTypeList()),
		"deny-list drifted from the documented set")
}

func permanentStreamErrorTypeList() []string {
	return []string{
		"invalid_request_error",
		"invalid_request",
		"invalid_api_key",
		"request_too_large",
		"authentication_error",
		"permission_error",
		"permission_denied",
		"account_error",
		"billing_error",
		"billing_hard_limit_reached",
		"insufficient_quota",
		"not_found_error",
		"model_not_found",
		"context_length_exceeded",
		"prompt_too_long",
		"content_policy_violation",
		"safety",
	}
}

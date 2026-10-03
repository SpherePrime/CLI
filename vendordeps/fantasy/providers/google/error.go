package google

import (
	"cmp"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/genai"
)

var googleContextPattern = regexp.MustCompile(`input token count.*?(\d+).*?exceeds.*?maximum.*?(\d+)`)

// permanentStreamErrorFragments are message fragments that mark a mid-stream
// Gemini failure as hopeless. Gemini reports mid-stream errors as free-form
// text with no structured type, so these stand in for the type deny-list
// fantasy.IsTransientStreamError applies elsewhere.
var permanentStreamErrorFragments = []string{
	"api key not valid",
	"permission denied",
	"billing",
	"quota exceeded",
	"content filter",
	"safety",
	"blocked",
}

func toProviderErr(err error) error {
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) {
		// Wrap transient transport failures so `.IsRetryable()` works.
		return fantasy.WrapTransportError(err)
	}

	providerErr := &fantasy.ProviderError{
		Message:      apiErr.Message,
		Title:        cmp.Or(fantasy.ErrorTitleForStatusCode(apiErr.Code), "provider request failed"),
		Cause:        err,
		StatusCode:   apiErr.Code,
		ResponseBody: []byte(apiErr.Message),
	}

	parseContextTooLargeError(apiErr.Message, providerErr)

	return providerErr
}

// toStreamErr classifies an error surfaced while reading an already
// established Gemini stream.
//
// toProviderErr handles the two shapes that carry their own retryability: a
// genai.APIError, which keeps its HTTP status for IsRetryable() to read, and
// a transport failure, which WrapTransportError marks. Anything it returns
// untouched is a stream that broke mid-flight with no status to judge it by,
// and such an error used to reach the user unclassified and unretried.
//
// The request was accepted and the break happened while producing the
// response, which is the same reasoning fantasy.IsTransientStreamError
// applies to an SSE error event elsewhere: retry, unless the message names a
// condition a retry cannot fix.
func toStreamErr(err error) error {
	providerErr := toProviderErr(err)

	var classified *fantasy.ProviderError
	if errors.As(providerErr, &classified) {
		return providerErr
	}

	streamErr := &fantasy.ProviderError{
		Title:          "provider stream error",
		Message:        err.Error(),
		Cause:          err,
		TransientError: !isPermanentStreamMessage(err.Error()),
	}

	// An oversized prompt is permanent no matter what the surrounding text
	// says, and the caller needs the token counts to act on it.
	parseContextTooLargeError(err.Error(), streamErr)
	streamErr.TransientError = !streamErr.ContextTooLargeErr

	return streamErr
}

func isPermanentStreamMessage(message string) bool {
	message = strings.ToLower(message)
	for _, fragment := range permanentStreamErrorFragments {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func parseContextTooLargeError(message string, providerErr *fantasy.ProviderError) {
	matches := googleContextPattern.FindStringSubmatch(message)
	if matches == nil {
		return
	}
	providerErr.ContextTooLargeErr = true
	providerErr.ContextUsedTokens, _ = strconv.Atoi(matches[1])
	providerErr.ContextMaxTokens, _ = strconv.Atoi(matches[2])
}

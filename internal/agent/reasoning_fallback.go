package agent

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openai"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openaicompat"
)

// reasoningCompatModel retries a call once without reasoning_effort when an
// OpenAI-style endpoint rejects the value (for example "none" or "max" on a
// model that does not support it), so a reasoning setting never blocks a run.
type reasoningCompatModel struct {
	fantasy.LanguageModel
	offRejected atomic.Bool
}

func newReasoningCompatModel(m fantasy.LanguageModel) fantasy.LanguageModel {
	if m == nil {
		return m
	}
	return &reasoningCompatModel{LanguageModel: m}
}

// isReasoningRejectionErr reports whether the provider refused the reasoning
// effort. Some proxies answer an unsupported "off" value with a bare 5xx, so
// server errors count too when reasoning was being switched off.
func isReasoningRejectionErr(err error, switchingOff bool) bool {
	var pe *fantasy.ProviderError
	if !errors.As(err, &pe) {
		return false
	}
	if switchingOff && pe.StatusCode >= http.StatusInternalServerError {
		return true
	}
	if pe.StatusCode != http.StatusBadRequest && pe.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	body := string(pe.ResponseBody)
	if body == "" {
		body = pe.Message
	}
	return strings.Contains(strings.ToLower(body), "reasoning")
}

func isOffEffort(effort *openai.ReasoningEffort) bool {
	return effort != nil && (*effort == openai.ReasoningEffortNone || *effort == openai.ReasoningEffortMinimal)
}

// reasoningEffortOf returns the reasoning effort set on an OpenAI-style call.
func reasoningEffortOf(call fantasy.Call) *openai.ReasoningEffort {
	for _, value := range call.ProviderOptions {
		switch typed := value.(type) {
		case *openai.ProviderOptions:
			if typed != nil && typed.ReasoningEffort != nil {
				return typed.ReasoningEffort
			}
		case *openai.ResponsesProviderOptions:
			if typed != nil && typed.ReasoningEffort != nil {
				return typed.ReasoningEffort
			}
		case *openaicompat.ProviderOptions:
			if typed != nil && typed.ReasoningEffort != nil {
				return typed.ReasoningEffort
			}
		}
	}
	return nil
}

// prepare drops an "off" effort the provider already refused once.
func (m *reasoningCompatModel) prepare(call fantasy.Call) fantasy.Call {
	if m.offRejected.Load() && isOffEffort(reasoningEffortOf(call)) {
		call, _ = withoutReasoningEffort(call)
	}
	return call
}

// withoutReasoningEffort returns the call with reasoning_effort removed, and
// whether there was one to remove.
func withoutReasoningEffort(call fantasy.Call) (fantasy.Call, bool) {
	options := make(fantasy.ProviderOptions, len(call.ProviderOptions))
	removed := false
	for key, value := range call.ProviderOptions {
		switch typed := value.(type) {
		case *openai.ProviderOptions:
			if typed != nil && typed.ReasoningEffort != nil {
				copied := *typed
				copied.ReasoningEffort = nil
				value, removed = &copied, true
			}
		case *openai.ResponsesProviderOptions:
			if typed != nil && typed.ReasoningEffort != nil {
				copied := *typed
				copied.ReasoningEffort = nil
				value, removed = &copied, true
			}
		case *openaicompat.ProviderOptions:
			if typed != nil && typed.ReasoningEffort != nil {
				copied := *typed
				copied.ReasoningEffort = nil
				value, removed = &copied, true
			}
		}
		options[key] = value
	}
	call.ProviderOptions = options
	return call, removed
}

func (m *reasoningCompatModel) retryCall(err error, call fantasy.Call) (fantasy.Call, bool) {
	switchingOff := isOffEffort(reasoningEffortOf(call))
	if !isReasoningRejectionErr(err, switchingOff) {
		return call, false
	}
	stripped, removed := withoutReasoningEffort(call)
	if removed && switchingOff {
		m.offRejected.Store(true)
	}
	if removed {
		slog.Info("Provider rejected the reasoning effort, retrying without it",
			"provider", m.Provider(), "model", m.Model(), "error", err)
	}
	return stripped, removed
}

// Generate implements fantasy.LanguageModel.
func (m *reasoningCompatModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	call = m.prepare(call)
	resp, err := m.LanguageModel.Generate(ctx, call)
	if err == nil {
		return resp, nil
	}
	if retry, ok := m.retryCall(err, call); ok {
		return m.LanguageModel.Generate(ctx, retry)
	}
	return resp, err
}

// Stream implements fantasy.LanguageModel. A rejected request fails before any
// part is produced, so the retry never duplicates forwarded output.
func (m *reasoningCompatModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	call = m.prepare(call)
	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		if retry, ok := m.retryCall(err, call); ok {
			return m.LanguageModel.Stream(ctx, retry)
		}
		return nil, err
	}
	return func(yield func(fantasy.StreamPart) bool) {
		forwarded := false
		retried := false
		inner(func(part fantasy.StreamPart) bool {
			if !forwarded && !retried && part.Type == fantasy.StreamPartTypeError {
				if retry, ok := m.retryCall(part.Error, call); ok {
					retried = true
					stream, err := m.LanguageModel.Stream(ctx, retry)
					if err != nil {
						yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
						return false
					}
					stream(yield)
					return false
				}
			}
			forwarded = true
			return yield(part)
		})
	}, nil
}

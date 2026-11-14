package primeai

import (
	"cmp"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/object"
)

type ProviderOption func(*providerConfig)

type providerConfig struct {
	baseURL      func() string
	accountToken func() string
	httpClient   *http.Client
	limits       *LimitsStore
	client       *Client
}

func WithProviderBaseURL(baseURL string) ProviderOption {
	return func(c *providerConfig) {
		if trimmed := strings.TrimSpace(baseURL); trimmed != "" {
			c.baseURL = func() string { return trimmed }
		}
	}
}

func WithProviderAccountToken(fn func() string) ProviderOption {
	return func(c *providerConfig) {
		if fn != nil {
			c.accountToken = fn
		}
	}
}

func WithProviderHTTPClient(hc *http.Client) ProviderOption {
	return func(c *providerConfig) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

func WithProviderLimitsStore(store *LimitsStore) ProviderOption {
	return func(c *providerConfig) {
		if store != nil {
			c.limits = store
		}
	}
}

func WithProviderClient(client *Client) ProviderOption {
	return func(c *providerConfig) {
		if client != nil {
			c.client = client
		}
	}
}

type Provider struct {
	client *Client
}

func NewProvider(opts ...ProviderOption) (fantasy.Provider, error) {
	cfg := providerConfig{
		baseURL:      BaseURL,
		accountToken: DefaultAccountToken,
		httpClient:   &http.Client{Timeout: 5 * time.Minute},
		limits:       defaultLimitsStore,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	client := cfg.client
	if client == nil {
		client = NewClient(
			cfg.accountToken,
			WithBaseURL(cfg.baseURL),
			WithHTTPClient(cfg.httpClient),
			WithLimitsStore(cfg.limits),
		)
	}
	return &Provider{client: client}, nil
}

func (p *Provider) Name() string {
	return Name
}

func (p *Provider) LanguageModel(_ context.Context, modelID string) (fantasy.LanguageModel, error) {
	return &languageModel{
		client:  p.client,
		modelID: cmp.Or(modelID, AutomaticModelID),
	}, nil
}

type languageModel struct {
	client  *Client
	modelID string
}

func (m *languageModel) Provider() string {
	return Name
}

func (m *languageModel) Model() string {
	return m.modelID
}

func (m *languageModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	resp, err := m.openCompletionStream(ctx, call)
	if err != nil {
		return nil, err
	}
	return func(yield func(fantasy.StreamPart) bool) {
		m.consumeSSE(ctx, resp.Body, yield)
	}, nil
}

func (m *languageModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	stream, err := m.Stream(ctx, call)
	if err != nil {
		return nil, err
	}

	var (
		textBuilder      strings.Builder
		reasoningBuilder strings.Builder
		toolCalls        []fantasy.ToolCallContent
		warnings         []fantasy.CallWarning
		usage            fantasy.Usage
		finishReason     = fantasy.FinishReasonUnknown
		providerMeta     fantasy.ProviderMetadata
		streamErr        error
	)

	for part := range stream {
		switch part.Type {
		case fantasy.StreamPartTypeWarnings:
			warnings = append(warnings, part.Warnings...)
		case fantasy.StreamPartTypeTextDelta:
			textBuilder.WriteString(part.Delta)
		case fantasy.StreamPartTypeReasoningDelta:
			reasoningBuilder.WriteString(part.Delta)
		case fantasy.StreamPartTypeToolCall:
			toolCalls = append(toolCalls, fantasy.ToolCallContent{
				ProviderExecuted: part.ProviderExecuted,
				ToolCallID:       part.ID,
				ToolName:         part.ToolCallName,
				Input:            part.ToolCallInput,
			})
		case fantasy.StreamPartTypeFinish:
			finishReason = part.FinishReason
			usage = part.Usage
			providerMeta = part.ProviderMetadata
		case fantasy.StreamPartTypeError:
			streamErr = part.Error
		}
	}

	if streamErr != nil {
		return nil, streamErr
	}

	var content fantasy.ResponseContent
	if reasoningBuilder.Len() > 0 {
		content = append(content, fantasy.ReasoningContent{Text: reasoningBuilder.String()})
	}
	if textBuilder.Len() > 0 {
		content = append(content, fantasy.TextContent{Text: textBuilder.String()})
	}
	for _, tc := range toolCalls {
		content = append(content, tc)
	}

	return &fantasy.Response{
		Content:          content,
		FinishReason:     finishReason,
		Usage:            usage,
		Warnings:         warnings,
		ProviderMetadata: providerMeta,
	}, nil
}

func (m *languageModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return object.GenerateWithTool(ctx, m, call)
}

func (m *languageModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return object.StreamWithTool(ctx, m, call)
}

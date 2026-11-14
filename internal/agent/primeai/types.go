package primeai

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	Name                = "primeai"
	DisplayName         = "PrimeAI"
	AutomaticModelID    = "__primeai_auto__"
	AutomaticModelName  = "Автоматически"
	defaultBaseURL      = "https://sphereprime.online"
	TypeProviderOptions = "primeai.options"
)

//go:embed provider.json
var embedded []byte

func init() {
	fantasy.RegisterProviderType(TypeProviderOptions, func(data []byte) (fantasy.ProviderOptionsData, error) {
		var opts Options
		if err := json.Unmarshal(data, &opts); err != nil {
			return nil, err
		}
		return &opts, nil
	})
}

func BaseURL() string {
	return normalizeBaseURL(cmp.Or(os.Getenv("PRIMEAI_URL"), defaultBaseURL))
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	for {
		next := trimmed
		for _, suffix := range []string{"/api/prime", "/api/plugin", "/v1"} {
			next = strings.TrimSuffix(next, suffix)
		}
		next = strings.TrimRight(next, "/")
		if next == trimmed {
			break
		}
		trimmed = next
	}
	if trimmed == "" {
		return defaultBaseURL
	}
	return trimmed
}

func IsLegacyBaseURL(raw string) bool {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return false
	}
	base := BaseURL()
	if trimmed == defaultBaseURL || trimmed == base {
		return true
	}
	for _, root := range []string{defaultBaseURL, base} {
		for _, suffix := range []string{"/v1", "/api/plugin"} {
			if trimmed == root+suffix {
				return true
			}
		}
	}
	return false
}

func Embedded() catwalk.Provider {
	var provider catwalk.Provider
	if err := json.Unmarshal(embedded, &provider); err != nil {
		slog.Error("Could not use embedded PrimeAI provider data", "err", err)
	}
	provider.APIEndpoint = BaseURL() + "/api/prime"
	return provider
}

type Options struct {
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
	ShowReasoning   *bool  `json:"showReasoning,omitempty"`
}

func (*Options) Options() {}

func (o Options) MarshalJSON() ([]byte, error) {
	type plain Options
	return fantasy.MarshalProviderType(TypeProviderOptions, plain(o))
}

func (o *Options) UnmarshalJSON(data []byte) error {
	type plain Options
	var p plain
	if err := fantasy.UnmarshalProviderType(data, &p); err != nil {
		return err
	}
	*o = Options(p)
	return nil
}

type LimitWindow struct {
	UsedTokens     int64   `json:"usedTokens"`
	LimitTokens    int64   `json:"limitTokens"`
	UsedRequests   int64   `json:"usedRequests"`
	LimitRequests  int64   `json:"limitRequests"`
	Percent        float64 `json:"percent"`
	ResetAt        string  `json:"resetAt,omitempty"`
	ResetInSeconds int64   `json:"resetInSeconds"`
}

type Limits struct {
	Available          bool         `json:"available"`
	PlanName           string       `json:"planName,omitempty"`
	CacheWeightPercent int          `json:"cacheWeightPercent"`
	FiveHour           *LimitWindow `json:"fiveHour,omitempty"`
	Weekly             *LimitWindow `json:"weekly,omitempty"`
	UpdatedAt          string       `json:"updatedAt,omitempty"`
	Error              string       `json:"error,omitempty"`
}

type PublicModel struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	RouteCount             int      `json:"routeCount"`
	Kind                   string   `json:"kind"`
	SupportsVision         bool     `json:"supportsVision,omitempty"`
	SupportsAttachments    bool     `json:"supportsAttachments,omitempty"`
	ContextWindow          int64    `json:"contextWindow,omitempty"`
	MaxContextLength       int64    `json:"maxContextLength,omitempty"`
	DefaultMaxTokens       int64    `json:"defaultMaxTokens,omitempty"`
	MaxOutputTokens        int64    `json:"maxOutputTokens,omitempty"`
	ReasoningEfforts       []string `json:"reasoningEfforts,omitempty"`
	DefaultReasoningEffort string   `json:"defaultReasoningEffort,omitempty"`
	PriceIn                *float64 `json:"priceIn,omitempty"`
	PriceOut               *float64 `json:"priceOut,omitempty"`
	AllowedTariffIDs       []string `json:"allowedTariffIds,omitempty"`
	AllowedTariffNames     []string `json:"allowedTariffNames,omitempty"`
	PlanLabel              string   `json:"planLabel,omitempty"`
	Locked                 bool     `json:"locked,omitempty"`
}

type ModelAccess struct {
	AllowedTariffIDs   []string `json:"allowedTariffIds,omitempty"`
	AllowedTariffNames []string `json:"allowedTariffNames,omitempty"`
	PlanLabel          string   `json:"planLabel,omitempty"`
	Locked             bool     `json:"locked,omitempty"`
}

func (a ModelAccess) Clone() ModelAccess {
	return ModelAccess{
		AllowedTariffIDs:   slices.Clone(a.AllowedTariffIDs),
		AllowedTariffNames: slices.Clone(a.AllowedTariffNames),
		PlanLabel:          a.PlanLabel,
		Locked:             a.Locked,
	}
}

type RouterImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type RouterContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *RouterImageURL `json:"image_url,omitempty"`
}

type RouterToolCall struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ArgumentsJson string `json:"argumentsJson"`
}

type RouterMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	Name       string           `json:"name,omitempty"`
	ToolCallID string           `json:"toolCallId,omitempty"`
	ToolCalls  []RouterToolCall `json:"toolCalls,omitempty"`
}

type RouterTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type RouterCompletionRequest struct {
	Model            string          `json:"model"`
	Messages         []RouterMessage `json:"messages"`
	SystemPrompt     string          `json:"systemPrompt,omitempty"`
	ConciseResponses *bool           `json:"conciseResponses,omitempty"`
	Tools            []RouterTool    `json:"tools,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	MaxTokens        *int64          `json:"maxTokens,omitempty"`
	ReasoningEffort  string          `json:"reasoningEffort,omitempty"`
	ShowReasoning    *bool           `json:"showReasoning,omitempty"`
	StrictModel      *bool           `json:"strictModel,omitempty"`
	ClientID         string          `json:"clientId,omitempty"`
}

type sseRouteEvent struct {
	Type         string `json:"type"`
	ProviderName string `json:"providerName"`
	ModelID      string `json:"modelId"`
	Substituted  bool   `json:"substituted"`
}

type sseDeltaEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

type sseToolCallEvent struct {
	Type string         `json:"type"`
	Call RouterToolCall `json:"call"`
}

type sseUsageEvent struct {
	Type              string `json:"type"`
	InputTokens       int64  `json:"inputTokens"`
	OutputTokens      int64  `json:"outputTokens"`
	CachedInputTokens int64  `json:"cachedInputTokens"`
}

type sseErrorEvent struct {
	Type      string `json:"type"`
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
	Detail    string `json:"detail"`
}

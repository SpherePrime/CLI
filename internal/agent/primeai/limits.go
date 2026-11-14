package primeai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LimitsStore struct {
	mu             sync.RWMutex
	cached         *Limits
	cachedTokenSum string
	nextID         uint64
	listeners      map[uint64]func(Limits)
}

func NewLimitsStore() *LimitsStore {
	return &LimitsStore{
		listeners: make(map[uint64]func(Limits)),
	}
}

func tokenDigest(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *LimitsStore) Cached() (Limits, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cached == nil {
		return Limits{}, false
	}
	return cloneLimits(*s.cached), true
}

func (s *LimitsStore) CachedForToken(token string) (Limits, bool) {
	sum := tokenDigest(token)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cached == nil {
		return Limits{}, false
	}
	if sum != "" && s.cachedTokenSum != "" && s.cachedTokenSum != sum {
		return Limits{}, false
	}
	return cloneLimits(*s.cached), true
}

func (s *LimitsStore) Set(l Limits) {
	s.setWithDigest("", l)
}

func (s *LimitsStore) SetForToken(token string, l Limits) {
	s.setWithDigest(tokenDigest(token), l)
}

func (s *LimitsStore) setWithDigest(digest string, l Limits) {
	cp := cloneLimits(l)
	s.mu.Lock()
	s.cached = &cp
	if digest != "" {
		s.cachedTokenSum = digest
	}
	listeners := make([]func(Limits), 0, len(s.listeners))
	for _, fn := range s.listeners {
		listeners = append(listeners, fn)
	}
	s.mu.Unlock()

	for _, fn := range listeners {
		fn(cloneLimits(cp))
	}
}

func (s *LimitsStore) Clear() {
	s.mu.Lock()
	s.cached = nil
	s.cachedTokenSum = ""
	s.mu.Unlock()
}

func (s *LimitsStore) Subscribe(fn func(Limits)) func() {
	if fn == nil {
		return func() {}
	}
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.listeners[id] = fn
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.listeners, id)
		s.mu.Unlock()
	}
}

var defaultLimitsStore = NewLimitsStore()

func DefaultLimitsStore() *LimitsStore {
	return defaultLimitsStore
}

func CachedLimits() (Limits, bool) {
	return defaultLimitsStore.Cached()
}

func CachedLimitsForToken(token string) (Limits, bool) {
	return defaultLimitsStore.CachedForToken(token)
}

func SetLimits(l Limits) {
	defaultLimitsStore.Set(l)
}

func ClearLimits() {
	defaultLimitsStore.Clear()
}

func SubscribeLimits(fn func(Limits)) func() {
	return defaultLimitsStore.Subscribe(fn)
}

type wireLimitWindow struct {
	UsedTokens     int64           `json:"usedTokens"`
	LimitTokens    int64           `json:"limitTokens"`
	UsedRequests   int64           `json:"usedRequests"`
	LimitRequests  int64           `json:"limitRequests"`
	Percent        float64         `json:"percent"`
	ResetAt        json.RawMessage `json:"resetAt"`
	ResetInSeconds int64           `json:"resetInSeconds"`
}

type wireLimitsPayload struct {
	Type               string           `json:"type,omitempty"`
	PlanName           string           `json:"planName,omitempty"`
	CacheWeightPercent int              `json:"cacheWeightPercent"`
	FiveHour           *wireLimitWindow `json:"fiveHour,omitempty"`
	Weekly             *wireLimitWindow `json:"weekly,omitempty"`
	UpdatedAt          string           `json:"updatedAt,omitempty"`
	Limits             *wireLimitsBody  `json:"limits,omitempty"`
}

type wireLimitsBody struct {
	PlanName           string           `json:"planName,omitempty"`
	CacheWeightPercent int              `json:"cacheWeightPercent"`
	FiveHour           *wireLimitWindow `json:"fiveHour,omitempty"`
	Weekly             *wireLimitWindow `json:"weekly,omitempty"`
	UpdatedAt          string           `json:"updatedAt,omitempty"`
}

func DecodeLimits(data []byte) (Limits, error) {
	var raw wireLimitsPayload
	if err := json.Unmarshal(data, &raw); err != nil {
		return Limits{}, err
	}
	planName := raw.PlanName
	cacheWeight := raw.CacheWeightPercent
	fiveHour := raw.FiveHour
	weekly := raw.Weekly
	updatedAt := raw.UpdatedAt
	if raw.Limits != nil {
		if raw.Limits.PlanName != "" {
			planName = raw.Limits.PlanName
		}
		if raw.Limits.CacheWeightPercent != 0 {
			cacheWeight = raw.Limits.CacheWeightPercent
		}
		if raw.Limits.FiveHour != nil {
			fiveHour = raw.Limits.FiveHour
		}
		if raw.Limits.Weekly != nil {
			weekly = raw.Limits.Weekly
		}
		if raw.Limits.UpdatedAt != "" {
			updatedAt = raw.Limits.UpdatedAt
		}
	}
	if updatedAt == "" {
		updatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return Limits{
		Available:          true,
		PlanName:           planName,
		CacheWeightPercent: cacheWeight,
		FiveHour:           convertWireWindow(fiveHour),
		Weekly:             convertWireWindow(weekly),
		UpdatedAt:          updatedAt,
	}, nil
}

func convertWireWindow(w *wireLimitWindow) *LimitWindow {
	if w == nil {
		return nil
	}
	return &LimitWindow{
		UsedTokens:     w.UsedTokens,
		LimitTokens:    w.LimitTokens,
		UsedRequests:   w.UsedRequests,
		LimitRequests:  w.LimitRequests,
		Percent:        w.Percent,
		ResetAt:        parseResetAt(w.ResetAt),
		ResetInSeconds: w.ResetInSeconds,
	}
}

func parseResetAt(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var asStr string
	if err := json.Unmarshal(raw, &asStr); err == nil {
		return asStr
	}
	var asNum float64
	if err := json.Unmarshal(raw, &asNum); err == nil && asNum > 0 {
		ms := int64(asNum)
		return time.UnixMilli(ms).UTC().Format(time.RFC3339)
	}
	return strings.Trim(trimmed, `"`)
}

func cloneLimits(l Limits) Limits {
	if l.FiveHour != nil {
		fh := *l.FiveHour
		l.FiveHour = &fh
	}
	if l.Weekly != nil {
		wk := *l.Weekly
		l.Weekly = &wk
	}
	return l
}

func (c *Client) FetchLimits(ctx context.Context) (Limits, error) {
	if strings.TrimSpace(c.accountToken()) == "" {
		return Limits{Available: false}, nil
	}

	resp, err := c.doWithSession(ctx, func(reqCtx context.Context) (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(reqCtx, http.MethodGet, c.base()+"/api/plugin/limits", nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Cache-Control", "no-store")
		return req, nil
	})
	if err != nil {
		return Limits{Available: false, Error: sanitizeErrorString(err.Error(), c.accountToken())}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return Limits{Available: false, Error: err.Error()}, err
	}
	if resp.StatusCode != http.StatusOK {
		msg := extractErrorMessage(body, "HTTP "+strconv.Itoa(resp.StatusCode))
		err := fmt.Errorf("PrimeAI limits request failed (%d): %s", resp.StatusCode, msg)
		return Limits{Available: false, Error: msg}, err
	}

	limits, err := DecodeLimits(body)
	if err != nil {
		return Limits{Available: false, Error: err.Error()}, err
	}
	if c.limits != nil {
		c.limits.SetForToken(c.accountToken(), limits)
	}
	return limits, nil
}

func FetchLimits(ctx context.Context) (Limits, error) {
	return DefaultClient().FetchLimits(ctx)
}

func FetchLimitsWithToken(ctx context.Context, accountToken string) (Limits, error) {
	token := strings.TrimSpace(accountToken)
	if token == "" {
		return Limits{Available: false}, nil
	}
	client := NewClient(
		func() string { return token },
		WithLimitsStore(defaultLimitsStore),
	)
	return client.FetchLimits(ctx)
}

func sanitizeErrorString(msg, secret string) string {
	if secret != "" {
		msg = strings.ReplaceAll(msg, secret, "[redacted]")
	}
	return msg
}

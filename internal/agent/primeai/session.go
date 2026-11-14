package primeai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	sessionRefreshSkew = 60 * time.Second
	maxSessionTokenLen = 1024
)

type SessionOption func(*SessionManager)

func WithSessionBaseURL(fn func() string) SessionOption {
	return func(m *SessionManager) {
		if fn != nil {
			m.baseURL = fn
		}
	}
}

func WithSessionHTTPClient(client *http.Client) SessionOption {
	return func(m *SessionManager) {
		if client != nil {
			m.httpClient = client
		}
	}
}

func WithSessionClock(now func() time.Time) SessionOption {
	return func(m *SessionManager) {
		if now != nil {
			m.now = now
		}
	}
}

type sessionExchangeCall struct {
	done  chan struct{}
	token string
	err   error
}

type SessionManager struct {
	mu           sync.Mutex
	baseURL      func() string
	accountToken func() string
	httpClient   *http.Client
	now          func() time.Time

	token       string
	expiresAt   time.Time
	lastAccount string
	inflight    *sessionExchangeCall
}

func NewSessionManager(accountToken func() string, opts ...SessionOption) *SessionManager {
	if accountToken == nil {
		accountToken = func() string { return "" }
	}
	m := &SessionManager{
		baseURL:      BaseURL,
		accountToken: accountToken,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		now:          time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

func (m *SessionManager) Invalidate(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if token == "" || m.token == token {
		m.token = ""
		m.expiresAt = time.Time{}
	}
}

func (m *SessionManager) Reset() {
	m.mu.Lock()
	m.token = ""
	m.expiresAt = time.Time{}
	m.lastAccount = ""
	m.mu.Unlock()
}

func (m *SessionManager) Get(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	acct := strings.TrimSpace(m.accountToken())
	if acct == "" {
		m.Reset()
		return "", &fantasy.ProviderError{
			StatusCode: http.StatusUnauthorized,
			AuthError:  true,
			Title:      "Unauthorized",
			Message:    "Prime account sign-in required",
		}
	}

	m.mu.Lock()
	if acct != m.lastAccount {
		m.token = ""
		m.expiresAt = time.Time{}
		m.lastAccount = acct
	}
	if m.token != "" && m.expiresAt.After(m.now().Add(sessionRefreshSkew)) {
		tok := m.token
		m.mu.Unlock()
		return tok, nil
	}
	if call := m.inflight; call != nil {
		m.mu.Unlock()
		select {
		case <-call.done:
			return call.token, call.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	call := &sessionExchangeCall{done: make(chan struct{})}
	m.inflight = call
	m.mu.Unlock()

	tok, exp, err := m.exchange(ctx, acct)

	m.mu.Lock()
	if err == nil && m.lastAccount == acct {
		m.token = tok
		m.expiresAt = exp
	}
	if m.inflight == call {
		m.inflight = nil
	}
	call.token = tok
	call.err = err
	close(call.done)
	m.mu.Unlock()

	return tok, err
}

func (m *SessionManager) exchange(ctx context.Context, acctToken string) (string, time.Time, error) {
	endpoint := normalizeBaseURL(m.baseURL()) + "/api/plugin/session"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("build session request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+acctToken)
	req.Header.Set("Cache-Control", "no-store")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, &fantasy.ProviderError{
			Title:          "PrimeAI connection failed",
			Message:        "Failed to connect to PrimeAI session endpoint",
			Cause:          err,
			TransientError: true,
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", time.Time{}, &fantasy.ProviderError{
			StatusCode: resp.StatusCode,
			AuthError:  true,
			Title:      "Unauthorized",
			Message:    extractErrorMessage(body, "Prime account authentication failed"),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, &fantasy.ProviderError{
			StatusCode:     resp.StatusCode,
			Title:          "PrimeAI connection failed",
			Message:        extractErrorMessage(body, fmt.Sprintf("PrimeAI session request failed (%d)", resp.StatusCode)),
			TransientError: resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests,
		}
	}

	var payload struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", time.Time{}, errors.New("invalid PrimeAI session response")
	}
	if payload.Token == "" || len(payload.Token) > maxSessionTokenLen || payload.ExpiresAt <= 0 {
		return "", time.Time{}, errors.New("invalid PrimeAI session payload")
	}

	exp := time.UnixMilli(payload.ExpiresAt)
	if !exp.After(m.now()) {
		return "", time.Time{}, errors.New("expired PrimeAI session returned")
	}
	return payload.Token, exp, nil
}

func extractErrorMessage(body []byte, fallback string) string {
	var parsed struct {
		Error string `json:"error"`
	}
	if len(body) > 0 && json.Unmarshal(body, &parsed) == nil && strings.TrimSpace(parsed.Error) != "" {
		return strings.TrimSpace(parsed.Error)
	}
	return fallback
}

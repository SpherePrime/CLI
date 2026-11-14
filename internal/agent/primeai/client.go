package primeai

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
)

type ClientOption func(*Client)

func WithBaseURL(fn func() string) ClientOption {
	return func(c *Client) {
		if fn != nil {
			c.baseURL = fn
		}
	}
}

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

func WithSessionManager(sm *SessionManager) ClientOption {
	return func(c *Client) {
		if sm != nil {
			c.session = sm
		}
	}
}

func WithLimitsStore(store *LimitsStore) ClientOption {
	return func(c *Client) {
		if store != nil {
			c.limits = store
		}
	}
}

type Client struct {
	baseURL       func() string
	accountToken  func() string
	httpClient    *http.Client
	session       *SessionManager
	limits        *LimitsStore
	modelAccessMu sync.RWMutex
	modelAccess   map[string]ModelAccess
}

func NewClient(accountToken func() string, opts ...ClientOption) *Client {
	if accountToken == nil {
		accountToken = func() string { return "" }
	}
	c := &Client{
		baseURL:      BaseURL,
		accountToken: accountToken,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		limits:       defaultLimitsStore,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.session == nil {
		c.session = NewSessionManager(
			c.accountToken,
			WithSessionBaseURL(c.baseURL),
			WithSessionHTTPClient(c.httpClient),
		)
	}
	return c
}

func (c *Client) base() string {
	return normalizeBaseURL(c.baseURL())
}

func (c *Client) Session() *SessionManager {
	return c.session
}

func (c *Client) Limits() *LimitsStore {
	return c.limits
}

func (c *Client) setModelAccess(access map[string]ModelAccess) {
	cloned := make(map[string]ModelAccess, len(access))
	for k, v := range access {
		cloned[k] = v.Clone()
	}
	c.modelAccessMu.Lock()
	c.modelAccess = cloned
	c.modelAccessMu.Unlock()
}

func (c *Client) ModelAccess(modelID string) (ModelAccess, bool) {
	c.modelAccessMu.RLock()
	defer c.modelAccessMu.RUnlock()
	if c.modelAccess == nil {
		return ModelAccess{}, false
	}
	access, ok := c.modelAccess[modelID]
	if !ok {
		return ModelAccess{}, false
	}
	return access.Clone(), true
}

func (c *Client) ModelAccessMap() map[string]ModelAccess {
	c.modelAccessMu.RLock()
	defer c.modelAccessMu.RUnlock()
	if len(c.modelAccess) == 0 {
		return map[string]ModelAccess{}
	}
	out := make(map[string]ModelAccess, len(c.modelAccess))
	for k, v := range c.modelAccess {
		out[k] = v.Clone()
	}
	return out
}

func (c *Client) doWithSession(ctx context.Context, buildReq func(context.Context) (*http.Request, error)) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	token, err := c.session.Get(ctx)
	if err != nil {
		return nil, err
	}

	req, err := buildReq(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	_ = resp.Body.Close()

	c.session.Invalidate(token)
	refreshed, err := c.session.Get(ctx)
	if err != nil {
		return nil, err
	}

	retryReq, err := buildReq(ctx)
	if err != nil {
		return nil, err
	}
	retryReq.Header.Set("Authorization", "Bearer "+refreshed)
	return c.httpClient.Do(retryReq)
}

var (
	defaultClientOnce sync.Once
	defaultClientInst *Client
	defaultAcctSvc    *account.Service
)

func DefaultAccountToken() string {
	defaultClientOnce.Do(initDefaultClient)
	if defaultAcctSvc == nil {
		return ""
	}
	return defaultAcctSvc.Token()
}

func DefaultClient() *Client {
	defaultClientOnce.Do(initDefaultClient)
	return defaultClientInst
}

func initDefaultClient() {
	defaultAcctSvc = account.NewService()
	defaultClientInst = NewClient(
		func() string { return defaultAcctSvc.Token() },
		WithLimitsStore(defaultLimitsStore),
	)
}

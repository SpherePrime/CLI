package config

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

type primeaiClient interface {
	Get(context.Context) (catwalk.Provider, error)
}

type primeaiAccessClient interface {
	GetWithAccess(context.Context) (catwalk.Provider, map[string]primeai.ModelAccess, error)
}

var _ syncer[catwalk.Provider] = (*primeaiSync)(nil)

var primeAccountToken = func() string {
	return account.NewService().Token()
}

var hasPrimeAccountToken = func() bool {
	return primeAccountToken() != ""
}

type primeaiSync struct {
	mu          sync.Mutex
	fetched     bool
	result      catwalk.Provider
	access      map[string]primeai.ModelAccess
	lastFetched time.Time
	now         func() time.Time
	err         error
	cache       cache[catwalk.Provider]
	client      primeaiClient
	autoupdate  bool
	init        atomic.Bool
}

func (s *primeaiSync) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func cloneModelAccessMap(src map[string]primeai.ModelAccess) map[string]primeai.ModelAccess {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]primeai.ModelAccess, len(src))
	for k, v := range src {
		dst[k] = v.Clone()
	}
	return dst
}

func (s *primeaiSync) Init(client primeaiClient, path string, autoupdate bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = client
	s.cache = newCache[catwalk.Provider](path)
	s.autoupdate = autoupdate
	s.fetched = false
	s.lastFetched = time.Time{}
	s.access = nil
	s.init.Store(true)
}

func (s *primeaiSync) SetClient(client primeaiClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = client
}

func (s *primeaiSync) ModelAccess(modelID string) (primeai.ModelAccess, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.access) == 0 {
		return primeai.ModelAccess{}, false
	}
	access, ok := s.access[modelID]
	if !ok {
		return primeai.ModelAccess{}, false
	}
	return access.Clone(), true
}

func (s *primeaiSync) ModelAccessMap() map[string]primeai.ModelAccess {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.access) == 0 {
		return map[string]primeai.ModelAccess{}
	}
	return cloneModelAccessMap(s.access)
}

func (s *primeaiSync) Get(ctx context.Context) (catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Get before Init")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.fetched {
		s.fetch(ctx)
		s.fetched = true
	}
	return s.result, s.err
}

func (s *primeaiSync) Refetch(ctx context.Context) (catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Refetch before Init")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetch(ctx)
	s.fetched = true
	return s.result, s.err
}

func (s *primeaiSync) RefetchIfStale(ctx context.Context, maxAge time.Duration) (catwalk.Provider, bool, error) {
	if !s.init.Load() {
		panic("called RefetchIfStale before Init")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fetched && !s.lastFetched.IsZero() && maxAge > 0 && s.clock().Sub(s.lastFetched) < maxAge {
		return s.result, false, s.err
	}
	s.fetch(ctx)
	s.fetched = true
	return s.result, true, s.err
}

func (s *primeaiSync) fetch(ctx context.Context) {
	s.err = nil
	if !s.autoupdate {
		s.result = primeai.Embedded()
		s.access = nil
		s.lastFetched = s.clock()
		return
	}

	fallback := s.result
	if fallback.ID == "" || len(fallback.Models) == 0 {
		cached, _, cachedErr := s.cache.Get()
		if cached.ID == "" || cachedErr != nil {
			fallback = primeai.Embedded()
		} else {
			fallback = sanitizePrimeAIProvider(cached)
		}
	}

	var (
		result catwalk.Provider
		access map[string]primeai.ModelAccess
		err    error
	)
	if ac, ok := s.client.(primeaiAccessClient); ok {
		result, access, err = ac.GetWithAccess(ctx)
	} else {
		result, err = s.client.Get(ctx)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Warn("PrimeAI provider not updated in time")
		s.result = fallback
		return
	}
	if err != nil {
		slog.Warn("Could not fetch the PrimeAI provider", "error", err)
		s.result = fallback
		return
	}
	if len(result.Models) == 0 {
		slog.Warn("PrimeAI did not return any models")
		s.result = fallback
		return
	}

	s.result = result
	s.access = cloneModelAccessMap(access)
	s.lastFetched = s.clock()
	s.err = s.cache.Store(result)
}

var (
	_ primeaiClient       = realPrimeAIClient{}
	_ primeaiAccessClient = realPrimeAIClient{}
)

type realPrimeAIClient struct {
	accountToken func() string
}

func (r realPrimeAIClient) GetWithAccess(ctx context.Context) (catwalk.Provider, map[string]primeai.ModelAccess, error) {
	base := primeai.Embedded()
	tokenFn := r.accountToken
	if tokenFn == nil {
		tokenFn = primeAccountToken
	}
	if tokenFn() == "" {
		return base, nil, nil
	}

	models, access, err := primeai.NewClient(tokenFn).FetchModelsWithAccess(ctx)
	if err != nil {
		return base, nil, err
	}
	if len(models) > 0 {
		base.Models = models
	}
	return base, access, nil
}

func (r realPrimeAIClient) Get(ctx context.Context) (catwalk.Provider, error) {
	provider, _, err := r.GetWithAccess(ctx)
	return provider, err
}

func PrimeAIModelAccess(modelID string) (primeai.ModelAccess, bool) {
	return primeaiSyncer.ModelAccess(modelID)
}

func PrimeAIModelAccessMap() map[string]primeai.ModelAccess {
	return primeaiSyncer.ModelAccessMap()
}

func RefreshPrimeAIIfStale(ctx context.Context, maxAge time.Duration) error {
	if !hasPrimeAccountToken() {
		return nil
	}
	client := realPrimeAIClient{
		accountToken: primeAccountToken,
	}
	if !primeaiSyncer.init.Load() {
		primeaiSyncer.Init(client, cachePathFor("primeai"), true)
	} else {
		primeaiSyncer.SetClient(client)
	}
	provider, _, err := primeaiSyncer.RefetchIfStale(ctx, maxAge)
	if err != nil {
		return err
	}
	provider = sanitizePrimeAIProvider(provider)
	UpdateProviderInList(provider)
	return nil
}

func sanitizePrimeAIProvider(provider catwalk.Provider) catwalk.Provider {
	if provider.ID == "" {
		return primeai.Embedded()
	}
	if strings.TrimSpace(provider.APIEndpoint) == "" || primeai.IsLegacyBaseURL(provider.APIEndpoint) {
		provider.APIEndpoint = primeai.BaseURL() + "/api/prime"
	}
	for i, model := range provider.Models {
		if model.ID == primeai.AutomaticModelID {
			if model.Name == "" || model.Name == "Автоматически (наилучший пинг)" {
				model.Name = primeai.AutomaticModelName
			}
			if model.ContextWindow == 0 || model.ContextWindow == 200000 {
				model.ContextWindow = 270000
			}
			provider.Models[i] = model
		}
	}
	return provider
}

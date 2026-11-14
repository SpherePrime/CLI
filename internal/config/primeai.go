package config

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

type primeaiClient interface {
	Get(context.Context) (catwalk.Provider, error)
}

var _ syncer[catwalk.Provider] = (*primeaiSync)(nil)

var hasPrimeAccountToken = func() bool {
	return account.NewService().Token() != ""
}

type primeaiSync struct {
	mu         sync.Mutex
	fetched    bool
	result     catwalk.Provider
	err        error
	cache      cache[catwalk.Provider]
	client     primeaiClient
	autoupdate bool
	init       atomic.Bool
}

func (s *primeaiSync) Init(client primeaiClient, path string, autoupdate bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = client
	s.cache = newCache[catwalk.Provider](path)
	s.autoupdate = autoupdate
	s.fetched = false
	s.init.Store(true)
}

func (s *primeaiSync) SetClient(client primeaiClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = client
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

func (s *primeaiSync) fetch(ctx context.Context) {
	s.err = nil
	if !s.autoupdate {
		s.result = primeai.Embedded()
		return
	}

	cached, _, cachedErr := s.cache.Get()
	if cached.ID == "" || cachedErr != nil {
		cached = primeai.Embedded()
	} else {
		cached = sanitizePrimeAIProvider(cached)
	}

	result, err := s.client.Get(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Warn("PrimeAI provider not updated in time")
		s.result = cached
		return
	}
	if err != nil {
		slog.Warn("Could not fetch the PrimeAI provider", "error", err)
		s.result = cached
		return
	}
	if len(result.Models) == 0 {
		slog.Warn("PrimeAI did not return any models")
		s.result = cached
		return
	}

	s.result = result
	s.err = s.cache.Store(result)
}

var _ primeaiClient = realPrimeAIClient{}

type realPrimeAIClient struct {
	accountToken func() string
}

func (r realPrimeAIClient) Get(ctx context.Context) (catwalk.Provider, error) {
	base := primeai.Embedded()
	tokenFn := r.accountToken
	if tokenFn == nil {
		tokenFn = func() string { return account.NewService().Token() }
	}
	if tokenFn() == "" {
		return base, nil
	}

	models, err := primeai.NewClient(tokenFn).FetchModels(ctx)
	if err != nil {
		return base, err
	}
	if len(models) > 0 {
		base.Models = models
	}
	return base, nil
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

package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/discover"
	"github.com/SpherePrime/CLI/internal/env"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// TestConfigureProvidersDoesNotBlockOnStoredModelRefresh pins the contract
// that a provider with usable stored models keeps them without waiting for
// the /models refresh: the load returns while the refresh is still parked in
// the provider's handler, and the refreshed list still reaches the discovery
// cache for the loads that follow.
func TestConfigureProvidersDoesNotBlockOnStoredModelRefresh(t *testing.T) {
	var (
		releaseOnce sync.Once
		release     = make(chan struct{})
		startedOnce sync.Once
		started     = make(chan struct{})
		requests    atomic.Int64
	)
	releaseRefresh := func() { releaseOnce.Do(func() { close(release) }) }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		startedOnce.Do(func() { close(started) })
		select {
		case <-release:
		case <-time.After(6 * time.Second):
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "stored-model"}, {"id": "fresh-model"}]}`))
	}))
	t.Cleanup(func() {
		releaseRefresh()
		server.Close()
	})

	discoverTrue := true
	cfg := &Config{
		Providers: csync.NewMapFrom(map[string]ProviderConfig{
			"custom": {
				APIKey:             "test-key",
				BaseURL:            server.URL + "/v1",
				Models:             []catwalk.Model{{ID: "stored-model", Name: "Stored"}},
				AutoDiscoverModels: &discoverTrue,
			},
		}),
	}
	cfg.setDefaults("/tmp", "")

	env := env.NewFromMap(map[string]string{})
	resolver := NewShellVariableResolver(env)

	begin := time.Now()
	require.NoError(t, cfg.configureProviders(context.Background(), testStore(cfg), env, resolver, []catwalk.Provider{}))
	elapsed := time.Since(begin)

	require.Less(t, elapsed, 2*time.Second, "a load with stored models must not wait for the model refresh")

	p, exists := cfg.Providers.Get("custom")
	require.True(t, exists)
	require.Len(t, p.Models, 1)
	require.Equal(t, "stored-model", p.Models[0].ID)
	require.Equal(t, "Stored", p.Models[0].Name)

	// The refresh is still parked in the handler, so it was genuinely in
	// flight while the load returned.
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the background refresh never contacted the provider")
	}
	require.Positive(t, requests.Load())

	releaseRefresh()

	key := discoveryCacheKey(discover.Config{
		ID:             "custom",
		BaseURL:        server.URL + "/v1",
		APIKey:         "test-key",
		ExistingModels: []catwalk.Model{{ID: "stored-model", Name: "Stored"}},
	}, catwalk.TypeOpenAICompat)

	var cached []catwalk.Model
	require.Eventually(t, func() bool {
		models, ok := cachedDiscovery(key)
		if !ok || len(models) != 2 {
			return false
		}
		cached = models
		return true
	}, 5*time.Second, 20*time.Millisecond, "the background refresh must populate the discovery cache")
	require.Equal(t, "stored-model", cached[0].ID)
	require.Equal(t, "fresh-model", cached[1].ID)
}

// TestRefreshDiscoveryInBackgroundCollapsesBursts verifies that concurrent
// loads of the same provider start a single refresh instead of one per load.
func TestRefreshDiscoveryInBackgroundCollapsesBursts(t *testing.T) {
	var (
		releaseOnce sync.Once
		release     = make(chan struct{})
		requests    atomic.Int64
	)
	releaseRefresh := func() { releaseOnce.Do(func() { close(release) }) }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "fresh-model"}]}`))
	}))
	t.Cleanup(func() {
		releaseRefresh()
		server.Close()
	})

	cfg := discover.Config{
		ID:             "custom",
		BaseURL:        server.URL + "/v1",
		APIKey:         "test-key",
		ExistingModels: []catwalk.Model{{ID: "stored-model"}},
	}
	key := discoveryCacheKey(cfg, catwalk.TypeOpenAICompat)
	resolver := NewShellVariableResolver(env.NewFromMap(map[string]string{}))

	const loads = 8
	var wg sync.WaitGroup
	for range loads {
		wg.Go(func() {
			refreshDiscoveryInBackground(key, cfg, catwalk.TypeOpenAICompat, resolver)
		})
	}
	wg.Wait()

	releaseRefresh()

	require.Eventually(t, func() bool {
		models, ok := cachedDiscovery(key)
		return ok && len(models) == 2
	}, 5*time.Second, 20*time.Millisecond, "the refresh must populate the discovery cache")
	require.Equal(t, int64(1), requests.Load(), "a burst of loads must start one refresh")
}

// TestRefreshDiscoveryInBackgroundKeepsCacheOnFailure verifies that a failed
// or empty refresh leaves the cache untouched so the next load retries. These
// configs carry no stored models: with stored models the provider's own list
// is part of the result, so an empty response is never an empty result.
func TestRefreshDiscoveryInBackgroundKeepsCacheOnFailure(t *testing.T) {
	for name, body := range map[string]string{
		"http status":  ``,
		"empty result": `{"data": []}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if body == "" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			t.Cleanup(server.Close)

			cfg := discover.Config{
				ID:      "custom",
				BaseURL: server.URL + "/v1",
				APIKey:  "test-key",
			}
			key := discoveryCacheKey(cfg, catwalk.TypeOpenAICompat)
			resolver := NewShellVariableResolver(env.NewFromMap(map[string]string{}))

			refreshDiscoveryInBackground(key, cfg, catwalk.TypeOpenAICompat, resolver)

			require.Never(t, func() bool {
				_, ok := cachedDiscovery(key)
				return ok
			}, 300*time.Millisecond, 20*time.Millisecond, "a failed refresh must not populate the cache")
		})
	}
}

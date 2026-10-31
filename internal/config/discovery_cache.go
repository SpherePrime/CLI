package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/discover"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

// discoveryCacheTTL keeps model lists found on one config load available to
// the loads that follow it, such as every workspace opened after startup.
const discoveryCacheTTL = 5 * time.Minute

type discoveryCacheEntry struct {
	models   []catwalk.Model
	storedAt time.Time
}

var discoveryCache sync.Map

func discoveryCacheKey(cfg discover.Config, providerType catwalk.Type) string {
	data, _ := json.Marshal(struct {
		Config discover.Config
		Type   catwalk.Type
	}{cfg, providerType})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cachedDiscovery(key string) ([]catwalk.Model, bool) {
	value, ok := discoveryCache.Load(key)
	if !ok {
		return nil, false
	}
	entry := value.(discoveryCacheEntry)
	if time.Since(entry.storedAt) > discoveryCacheTTL {
		discoveryCache.Delete(key)
		return nil, false
	}
	return entry.models, true
}

func storeDiscovery(key string, models []catwalk.Model) {
	discoveryCache.Store(key, discoveryCacheEntry{models: models, storedAt: time.Now()})
}

// refreshInFlight holds the cache keys with a refresh already running, so a
// burst of loads starts at most one refresh per provider.
var refreshInFlight sync.Map

// refreshDiscoveryInBackground refreshes the cached model list for key
// without blocking the caller. Everything the refresh needs is resolved
// through resolver before the goroutine starts, so the goroutine never
// touches the config store's resolver: it receives resolved values and an
// identity resolver, and it leaves the cache untouched unless it finds
// models, so the next load retries after a failure.
func refreshDiscoveryInBackground(key string, cfg discover.Config, providerType catwalk.Type, resolver VariableResolver) {
	if _, loaded := refreshInFlight.LoadOrStore(key, struct{}{}); loaded {
		return
	}

	cfg.BaseURL, _ = resolver.ResolveValue(cfg.BaseURL)
	cfg.APIKey, _ = resolver.ResolveValue(cfg.APIKey)
	if len(cfg.ExtraHeaders) > 0 {
		headers := make(map[string]string, len(cfg.ExtraHeaders))
		for name, value := range cfg.ExtraHeaders {
			resolved, err := resolver.ResolveValue(value)
			if err != nil || resolved == "" {
				continue
			}
			headers[name] = resolved
		}
		cfg.ExtraHeaders = headers
	}

	go func() {
		defer refreshInFlight.Delete(key)
		ctx, cancel := context.WithTimeout(context.Background(), discover.ProviderDiscoveryTimeout)
		defer cancel()
		models, err := discover.DiscoverModels(ctx, cfg, identityResolver{})
		if err != nil || len(models) == 0 {
			return
		}
		if enricher := discover.GetEnricher(string(providerType)); enricher != nil {
			models, _ = enricher.EnrichModels(ctx, cfg, identityResolver{}, models)
		}
		if len(models) == 0 {
			return
		}
		storeDiscovery(key, models)
	}()
}

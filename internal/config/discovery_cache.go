package config

import (
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

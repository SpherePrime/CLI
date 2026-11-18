package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

type discoveredModelUpdate struct {
	id     string
	models []catwalk.Model
}

func discoveredModelsDiffer(stored, merged []catwalk.Model) bool {
	if len(stored) != len(merged) {
		return true
	}
	for i := range stored {
		if !modelMetadataEqual(stored[i], merged[i]) {
			return true
		}
	}
	return false
}

func modelMetadataEqual(a, b catwalk.Model) bool {
	if a.ID != b.ID ||
		a.ContextWindow != b.ContextWindow ||
		a.DefaultMaxTokens != b.DefaultMaxTokens ||
		a.SupportsImages != b.SupportsImages ||
		a.CanReason != b.CanReason ||
		a.DefaultReasoningEffort != b.DefaultReasoningEffort ||
		!slices.Equal(a.ReasoningLevels, b.ReasoningLevels) {
		return false
	}
	return providerOptionsEqual(a.Options.ProviderOptions, b.Options.ProviderOptions)
}

func providerOptionsEqual(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func modelCapabilitySignature(models []catwalk.Model, check func(catwalk.Model) bool) []string {
	var ids []string
	for _, m := range models {
		if check(m) {
			ids = append(ids, m.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func modelCapabilitiesDiffer(a, b []catwalk.Model) bool {
	return !slices.Equal(modelCapabilitySignature(a, IsWebSearchModel), modelCapabilitySignature(b, IsWebSearchModel)) ||
		!slices.Equal(modelCapabilitySignature(a, IsImageOutputModel), modelCapabilitySignature(b, IsImageOutputModel))
}

func applyDiscoveredModelsToConfig(cfg *Config, id string, models []catwalk.Model) (bool, bool) {
	if cfg == nil || cfg.Providers == nil || len(models) == 0 {
		return false, false
	}
	existing, ok := cfg.Providers.Get(id)
	if !ok || existing.Disable {
		return false, false
	}
	if !discoveredModelsDiffer(existing.Models, models) {
		return false, false
	}
	capabilitiesChanged := modelCapabilitiesDiffer(existing.Models, models)
	existing.Models = slices.Clone(models)
	cfg.Providers.Set(id, existing)
	return true, capabilitiesChanged
}

func (s *ConfigStore) applyBackgroundDiscoveredModels(fallbackCfg *Config, id string, models []catwalk.Model) {
	if s == nil {
		applyDiscoveredModelsToConfig(fallbackCfg, id, models)
		return
	}
	s.writeMu.Lock()
	cfg := s.Config()
	if cfg == nil {
		cfg = fallbackCfg
	}
	changed, capabilitiesChanged := applyDiscoveredModelsToConfig(cfg, id, models)
	if !changed {
		s.writeMu.Unlock()
		return
	}
	if scope, ok := s.discoveredModelScope(id); ok {
		if dir, err := s.configDirForScope(scope); err == nil {
			if err := s.writeConfigFields(scope, map[string]any{"providers." + id + ".models": slices.Clone(models)}); err != nil {
				slog.Warn("Failed to persist background discovered models", "provider", id, "error", err)
			} else {
				s.captureStalenessSnapshot(append(slices.Clone(s.loadedPaths), configFilesForDir(dir)...))
				slog.Info("Persisted background discovered models", "provider", id, "count", len(models))
			}
		}
	}
	s.writeMu.Unlock()

	if capabilitiesChanged {
		if hook := s.backgroundDiscoveryHook(); hook != nil {
			go hook()
		}
	}
}

func (s *ConfigStore) queueDiscoveredModels(id string, models []catwalk.Model) {
	s.pendingDiscovery = append(s.pendingDiscovery, discoveredModelUpdate{id: id, models: models})
}

func (s *ConfigStore) flushDiscoveredModels() {
	if len(s.pendingDiscovery) == 0 {
		return
	}
	updates := s.pendingDiscovery
	s.pendingDiscovery = nil
	for _, update := range updates {
		scope, ok := s.discoveredModelScope(update.id)
		if !ok {
			continue
		}
		dir, err := s.configDirForScope(scope)
		if err != nil {
			continue
		}
		if err := s.writeConfigFields(scope, map[string]any{"providers." + update.id + ".models": update.models}); err != nil {
			slog.Warn("Failed to persist discovered models", "provider", update.id, "error", err)
			continue
		}
		s.captureStalenessSnapshot(append(slices.Clone(s.loadedPaths), configFilesForDir(dir)...))
		slog.Info("Persisted discovered models", "provider", update.id, "count", len(update.models))
	}
}

func (s *ConfigStore) discoveredModelScope(id string) (Scope, bool) {
	if dir, err := s.configDirForScope(ScopeWorkspace); err == nil && providerDefinedInProvidersFile(dir, id) {
		return ScopeWorkspace, true
	}
	if dir, err := s.configDirForScope(ScopeGlobal); err == nil && providerDefinedInProvidersFile(dir, id) {
		return ScopeGlobal, true
	}
	return 0, false
}

func providerDefinedInProvidersFile(dir, id string) bool {
	data, err := os.ReadFile(filepath.Join(dir, providersSectionFile))
	if err != nil {
		return false
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return false
	}
	entries, ok := document["providers"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = entries[id]
	return ok
}

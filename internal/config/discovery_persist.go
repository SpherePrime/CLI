package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
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
		if stored[i].ID != merged[i].ID {
			return true
		}
	}
	return false
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

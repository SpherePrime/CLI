package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/internal/lock"
)

type ProviderEdit struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	BaseURL        string           `json:"base_url"`
	Type           string           `json:"type"`
	DiscoverModels bool             `json:"discover_models"`
	Models         []map[string]any `json:"models"`
}

func (p ProviderEdit) Validate() error {
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(p.ID) {
		return fmt.Errorf("ID провайдера: латиница, цифры, _ и -")
	}
	address, err := url.Parse(p.BaseURL)
	if err != nil || address.Host == "" || (address.Scheme != "http" && address.Scheme != "https") {
		return fmt.Errorf("Нужен HTTP/HTTPS адрес API")
	}
	switch p.Type {
	case "openai", "openai-compat", "anthropic", "ollama":
	default:
		return fmt.Errorf("unsupported provider type")
	}
	seen := map[string]bool{}
	for _, model := range p.Models {
		id, _ := model["id"].(string)
		if strings.TrimSpace(id) == "" || seen[id] {
			return fmt.Errorf("У каждой модели должен быть уникальный ID")
		}
		seen[id] = true
		for _, field := range []string{"context_window", "default_max_tokens"} {
			if value, exists := model[field]; exists {
				number, ok := value.(float64)
				if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > 9007199254740991 || number != math.Trunc(number) {
					return fmt.Errorf("Модель «%s»: %s должен быть целым неотрицательным числом", id, field)
				}
			}
		}
		context, _ := model["context_window"].(float64)
		tokens, _ := model["default_max_tokens"].(float64)
		if context > 0 && tokens > context {
			return fmt.Errorf("Модель «%s»: лимит ответа превышает контекст", id)
		}
	}
	if !p.DiscoverModels && len(p.Models) == 0 {
		return fmt.Errorf("Добавьте хотя бы одну модель или включите автопоиск моделей")
	}
	return nil
}

func (s *ConfigStore) SaveProvider(scope Scope, originalID string, edit ProviderEdit, apiKey *string) error {
	if scope != ScopeGlobal && scope != ScopeWorkspace {
		return fmt.Errorf("invalid config scope")
	}
	if err := edit.Validate(); err != nil {
		return err
	}
	dir, err := s.configDirForScope(scope)
	if err != nil {
		return err
	}
	renamed := originalID != "" && originalID != edit.ID
	if (originalID == "" || renamed) && scope == ScopeWorkspace && s.Config().Providers != nil {
		if _, exists := s.Config().Providers.Get(edit.ID); exists {
			return fmt.Errorf("Провайдер с таким ID уже существует")
		}
	}
	err = s.editProviderFiles(dir, func(documents map[string]map[string]any) error {
		providers := map[string]any{}
		for _, name := range []string{legacyConfigName(), providersSectionFile} {
			if entries, ok := documents[name]["providers"].(map[string]any); ok {
				for id, value := range entries {
					if previous, ok := providers[id].(map[string]any); ok {
						if next, ok := value.(map[string]any); ok {
							for field, item := range next {
								previous[field] = item
							}
							continue
						}
					}
					providers[id] = value
				}
			}
		}
		_, targetExists := providers[edit.ID]
		if (originalID == "" || renamed) && targetExists {
			return fmt.Errorf("Провайдер с таким ID уже существует")
		}
		source, exists := providers[originalID].(map[string]any)
		if renamed && !exists {
			return fmt.Errorf("Переименуйте провайдера в области, где он сохранён")
		}
		if source == nil {
			source = map[string]any{}
			if scope == ScopeWorkspace && originalID != "" && s.Config().Providers != nil {
				if inherited, ok := s.Config().Providers.Get(originalID); ok {
					encoded, err := json.Marshal(inherited)
					if err != nil {
						return err
					}
					if err := json.Unmarshal(encoded, &source); err != nil {
						return err
					}
				}
			}
		}
		source["id"] = edit.ID
		source["name"] = edit.Name
		source["base_url"] = edit.BaseURL
		source["type"] = edit.Type
		source["discover_models"] = edit.DiscoverModels
		source["models"] = mergeProviderModels(source["models"], edit.Models)
		if apiKey != nil && strings.TrimSpace(*apiKey) != "" {
			source["api_key"] = *apiKey
		}
		if documents[providersSectionFile] == nil {
			documents[providersSectionFile] = map[string]any{}
		}
		sectionProviders, _ := documents[providersSectionFile]["providers"].(map[string]any)
		if sectionProviders == nil {
			sectionProviders = map[string]any{}
			documents[providersSectionFile]["providers"] = sectionProviders
		}
		sectionProviders[edit.ID] = source
		if renamed {
			for _, document := range documents {
				if entries, ok := document["providers"].(map[string]any); ok {
					delete(entries, originalID)
				}
				renameProviderReferences(document, originalID, edit.ID)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.ReloadProviderConfiguration(originalID, edit.ID)
}

func (s *ConfigStore) ReloadProviderConfiguration(originalID, nextID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if originalID != "" && originalID != nextID {
		for kind, model := range s.overrides.Models {
			if model.Provider == originalID {
				model.Provider = nextID
				s.overrides.Models[kind] = model
			}
		}
	}
	if s.workingDir == "" {
		return nil
	}
	if err := s.reloadFromDiskLocked(context.Background()); err != nil {
		return fmt.Errorf("Настройки сохранены, но не применены: %w", err)
	}
	return nil
}

func (s *ConfigStore) ValidateGlobalProviderRename(originalID string) error {
	dir, err := s.configDirForScope(ScopeWorkspace)
	if errors.Is(err, ErrNoWorkspaceConfig) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, name := range []string{legacyConfigName(), providersSectionFile, modelsSectionFile} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		var document map[string]any
		if err := json.Unmarshal(data, &document); err != nil {
			return err
		}
		if entries, ok := document["providers"].(map[string]any); ok {
			if _, exists := entries[originalID]; exists {
				return fmt.Errorf("В открытом проекте сохранены настройки этого провайдера. Сначала измените их в области проекта")
			}
		}
		before, _ := json.Marshal(document)
		renameProviderReferences(document, originalID, originalID+"-renamed")
		after, _ := json.Marshal(document)
		if string(before) != string(after) {
			return fmt.Errorf("В открытом проекте сохранены ссылки на этот провайдер. Сначала измените модель в области проекта")
		}
	}
	return nil
}

func mergeProviderModels(current any, edited []map[string]any) []map[string]any {
	existing := map[string]map[string]any{}
	if models, ok := current.([]any); ok {
		for _, item := range models {
			if model, ok := item.(map[string]any); ok {
				if id, ok := model["id"].(string); ok {
					existing[id] = model
				}
			}
		}
	}
	result := make([]map[string]any, 0, len(edited))
	for _, model := range edited {
		id, _ := model["id"].(string)
		merged := existing[id]
		if merged == nil {
			merged = map[string]any{}
		}
		delete(merged, "context_window")
		delete(merged, "default_max_tokens")
		for field, value := range model {
			merged[field] = value
		}
		result = append(result, merged)
	}
	return result
}

func renameProviderReferences(document map[string]any, originalID, nextID string) {
	var rename func(any)
	rename = func(value any) {
		switch item := value.(type) {
		case map[string]any:
			if item["provider"] == originalID {
				item["provider"] = nextID
			}
			for _, child := range item {
				rename(child)
			}
		case []any:
			for _, child := range item {
				rename(child)
			}
		}
	}
	for _, key := range []string{"models", "recent_models", "agents"} {
		rename(document[key])
	}
	if settings, ok := document["model_settings"].(map[string]any); ok {
		for key, value := range settings {
			if strings.HasPrefix(key, originalID+"|") {
				delete(settings, key)
				settings[nextID+strings.TrimPrefix(key, originalID)] = value
			}
		}
	}
}

func (s *ConfigStore) editProviderFiles(dir string, mutate func(map[string]map[string]any) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	names := []string{legacyConfigName(), providersSectionFile, modelsSectionFile}
	slices.Sort(names)
	releases := []func(){}
	defer func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}()
	originals := map[string][]byte{}
	documents := map[string]map[string]any{}
	for _, name := range names {
		path := filepath.Join(dir, name)
		ctx, cancel := context.WithTimeout(context.Background(), configLockDeadline)
		release, err := lock.File(ctx, path+".lock")
		cancel()
		if err != nil {
			return err
		}
		releases = append(releases, release)
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		document := map[string]any{}
		if err == nil {
			originals[name] = data
			if err := json.Unmarshal(data, &document); err != nil {
				return err
			}
			if document == nil {
				return fmt.Errorf("invalid config document: %s", name)
			}
		}
		documents[name] = document
	}
	if err := mutate(documents); err != nil {
		return err
	}
	encoded := map[string][]byte{}
	for _, name := range names {
		data, err := json.Marshal(documents[name])
		if err != nil {
			return err
		}
		if len(documents[name]) > 0 || originals[name] != nil {
			encoded[name] = data
		}
	}
	written := []string{}
	for _, name := range names {
		data, exists := encoded[name]
		if !exists {
			continue
		}
		if err := atomicWriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			for i := len(written) - 1; i >= 0; i-- {
				previous := written[i]
				var restoreErr error
				if original, exists := originals[previous]; exists {
					restoreErr = atomicWriteFile(filepath.Join(dir, previous), original, 0600)
				} else {
					restoreErr = os.Remove(filepath.Join(dir, previous))
				}
				err = errors.Join(err, restoreErr)
			}
			return err
		}
		written = append(written, name)
	}
	return nil
}

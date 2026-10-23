package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/internal/lock"
)

type providerFileMutation func(map[string]map[string]any) error

func lockProviderStores(source *ConfigStore, workspaces []*ConfigStore) ([]*ConfigStore, func()) {
	seen := map[*ConfigStore]bool{}
	stores := make([]*ConfigStore, 0, len(workspaces)+1)
	for _, store := range append([]*ConfigStore{source}, workspaces...) {
		if !seen[store] {
			seen[store] = true
			stores = append(stores, store)
		}
	}
	slices.SortFunc(stores, func(left, right *ConfigStore) int {
		return strings.Compare(fmt.Sprintf("%p", left), fmt.Sprintf("%p", right))
	})
	for _, store := range stores {
		store.writeMu.Lock()
	}
	return stores, func() {
		for i := len(stores) - 1; i >= 0; i-- {
			stores[i].writeMu.Unlock()
		}
	}
}

func (s *ConfigStore) editProviderFiles(dir string, mutate providerFileMutation) error {
	return s.editProviderDirectories(map[string]providerFileMutation{dir: mutate})
}

func (s *ConfigStore) editProviderFilesWithReferences(dir string, scope Scope, originalID, nextID string, workspaces []*ConfigStore, mutate providerFileMutation) error {
	mutations := map[string]providerFileMutation{filepath.Clean(dir): mutate}
	if scope == ScopeGlobal && originalID != "" && originalID != nextID {
		for _, workspace := range workspaces {
			workspaceDir, err := workspace.configDirForScope(ScopeWorkspace)
			if errors.Is(err, ErrNoWorkspaceConfig) {
				continue
			}
			if err != nil {
				return err
			}
			workspaceDir = filepath.Clean(workspaceDir)
			if workspaceDir == filepath.Clean(dir) {
				continue
			}
			mutations[workspaceDir] = func(documents map[string]map[string]any) error {
				for _, document := range documents {
					if providers, ok := document["providers"].(map[string]any); ok {
						if _, exists := providers[originalID]; exists {
							return fmt.Errorf("В открытом проекте сохранены настройки этого провайдера. Сначала измените их в области проекта")
						}
						if _, exists := providers[nextID]; exists {
							return fmt.Errorf("Провайдер с новым ID уже существует в открытом проекте")
						}
					}
					renameProviderReferences(document, originalID, nextID)
				}
				return nil
			}
		}
	}
	return s.editProviderDirectories(mutations)
}

func (s *ConfigStore) editProviderDirectories(mutations map[string]providerFileMutation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	directories := make([]string, 0, len(mutations))
	for dir := range mutations {
		directories = append(directories, dir)
	}
	slices.Sort(directories)
	names := []string{legacyConfigName(), providersSectionFile, modelsSectionFile}
	slices.Sort(names)
	releases := []func(){}
	defer func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}()
	originals := map[string][]byte{}
	encoded := map[string][]byte{}
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
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
				originals[path] = data
				if err := json.Unmarshal(data, &document); err != nil {
					return err
				}
				if document == nil {
					return fmt.Errorf("invalid config document: %s", name)
				}
			}
			documents[name] = document
		}
		if err := mutations[dir](documents); err != nil {
			return err
		}
		for _, name := range names {
			path := filepath.Join(dir, name)
			data, err := json.Marshal(documents[name])
			if err != nil {
				return err
			}
			if len(documents[name]) > 0 || originals[path] != nil {
				encoded[path] = data
			}
		}
	}
	return persistProviderFiles(originals, encoded, atomicWriteFile)
}

func persistProviderFiles(originals, encoded map[string][]byte, write func(string, []byte, os.FileMode) error) error {
	paths := make([]string, 0, len(encoded))
	for path := range encoded {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	written := []string{}
	for _, path := range paths {
		if err := write(path, encoded[path], 0600); err != nil {
			for i := len(written) - 1; i >= 0; i-- {
				previous := written[i]
				var restoreErr error
				if original, exists := originals[previous]; exists {
					restoreErr = write(previous, original, 0600)
				} else {
					restoreErr = os.Remove(previous)
				}
				err = errors.Join(err, restoreErr)
			}
			return err
		}
		written = append(written, path)
	}
	return nil
}

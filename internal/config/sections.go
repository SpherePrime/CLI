package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Configuration is stored as one JSON file per section in the config
// directory, instead of a single monolithic prime.json. Routing is decided by
// the dotted key of a write, so every reader and writer agrees on where a
// value lives without being told twice.
const (
	providersSectionFile = "providers.json"
	modelsSectionFile    = "models.json"
	mcpSectionFile       = "mcp.json"
	lspSectionFile       = "lsp.json"
	skillsSectionFile    = "skills.json"
	optionsSectionFile   = "options.json"
	pluginsSectionFile   = "plugins.json"
)

// legacyConfigName is the pre-split single config file. It is still read, so
// hand-written setups keep working until it is migrated into sections.
func legacyConfigName() string {
	return fmt.Sprintf("%s.json", appName)
}

// sectionFileOrder lists the section files in load order, applied after the
// catch-all prime.json of the same directory so a value that legitimately
// appears in two places resolves predictably.
var sectionFileOrder = []string{
	providersSectionFile,
	modelsSectionFile,
	mcpSectionFile,
	lspSectionFile,
	skillsSectionFile,
	optionsSectionFile,
	pluginsSectionFile,
}

// skillOptionKeys are the option fields stored in skills.json. They stay under
// "options" in the schema; only their file changes.
var skillOptionKeys = []string{
	"options.skills_paths",
	"options.disabled_skills",
	"skills",
}

// sectionFileFor returns the file that stores the given dotted config key.
func sectionFileFor(key string) string {
	root := key
	if dot := strings.Index(key, "."); dot >= 0 {
		root = key[:dot]
	}

	for _, skillKey := range skillOptionKeys {
		if key == skillKey || strings.HasPrefix(key, skillKey+".") {
			return skillsSectionFile
		}
	}

	switch root {
	case "providers":
		return providersSectionFile
	case "models", "recent_models", "model_settings":
		return modelsSectionFile
	case "mcp":
		return mcpSectionFile
	case "plugins":
		return pluginsSectionFile
	case "lsp":
		return lspSectionFile
	case "options", "tools", "permissions", "hooks", "env":
		return optionsSectionFile
	default:
		// Everything else, including the editor-facing $schema pointer, stays
		// in the catch-all file so hand-written configs keep working.
		return legacyConfigName()
	}
}

// sectionPath returns the file inside dir that owns key.
func sectionPath(dir, key string) string {
	return filepath.Join(dir, sectionFileFor(key))
}

// legacyConfigPath returns the pre-split single config file for a directory.
func legacyConfigPath(dir string) string {
	return filepath.Join(dir, legacyConfigName())
}

// migratedConfigPath returns the name a legacy config file is renamed to once
// its contents have been distributed across the section files.
func migratedConfigPath(dir string) string {
	return legacyConfigPath(dir) + ".bak"
}

// fileHasContent reports whether path exists and holds bytes worth parsing.
func fileHasContent(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Size() > 0
}

// sectionFileNames returns the section file names, for callers that scan a
// directory by name instead of resolving a single key.
func sectionFileNames() []string {
	return slices.Clone(sectionFileOrder)
}

// configFilesForDir returns the config files of a directory in load order: the
// catch-all prime.json first, then each section file that exists. Sections are
// applied last because they are what Prime writes, and migrating a legacy file
// removes the keys it copies, so anything still in prime.json is a key with no
// section of its own.
func configFilesForDir(dir string) []string {
	if dir == "" {
		return nil
	}
	paths := make([]string, 0, len(sectionFileOrder)+1)
	paths = append(paths, legacyConfigPath(dir))
	for _, name := range sectionFileOrder {
		if path := filepath.Join(dir, name); fileHasContent(path) {
			paths = append(paths, path)
		}
	}
	return paths
}

// allConfigPathsIn returns every config path a directory could use, existing
// or not, in load order. Used for change tracking so a newly created section
// file counts as a change.
func allConfigPathsIn(dir string) []string {
	if dir == "" {
		return nil
	}
	paths := make([]string, 0, len(sectionFileOrder)+1)
	paths = append(paths, legacyConfigPath(dir))
	for _, name := range sectionFileOrder {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}

// GlobalConfigDir returns the directory holding the user's global config
// sections: $XDG_CONFIG_HOME/prime, or ~/.config/prime. Every setting Prime
// persists lives here, one JSON file per section.
func GlobalConfigDir() string {
	return filepath.Dir(GlobalConfig())
}

// GlobalDataDir returns the machine-owned data directory (state, logs, locks).
func GlobalDataDir() string {
	return filepath.Dir(GlobalConfigData())
}

// GlobalProvidersFile returns the file holding provider credentials. It exists
// so user-facing text can name the exact file a key lands in.
func GlobalProvidersFile() string {
	return sectionPath(GlobalConfigDir(), "providers")
}

// sectionPathsForKeys returns the section files that own the given keys, in a
// stable order, so a multi-key write touches each file exactly once.
func sectionPathsForKeys(dir string, keys []string) []string {
	order := make([]string, 0, len(keys))
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		path := sectionPath(dir, key)
		if seen[path] {
			continue
		}
		seen[path] = true
		order = append(order, path)
	}
	slices.Sort(order)
	return order
}

// keysForSectionFile returns the keys from keys that belong to file.
func keysForSectionFile(dir, file string, keys []string) []string {
	var result []string
	for _, key := range keys {
		if sectionPath(dir, key) == file {
			result = append(result, key)
		}
	}
	slices.Sort(result)
	return result
}

// configDirForScope returns the directory whose JSON files back a scope. The
// scope's anchor file defines the directory, so overriding the anchor (as tests
// do) redirects every section write with it.
func (s *ConfigStore) configDirForScope(scope Scope) (string, error) {
	switch scope {
	case ScopeWorkspace:
		if s.workspacePath == "" {
			return "", ErrNoWorkspaceConfig
		}
		return filepath.Dir(s.workspacePath), nil
	default:
		if s.globalDataPath == "" {
			return "", ErrNoWorkspaceConfig
		}
		return filepath.Dir(s.globalDataPath), nil
	}
}

// configPathForKey returns the file that stores key for the given scope.
func (s *ConfigStore) configPathForKey(scope Scope, key string) (string, error) {
	dir, err := s.configDirForScope(scope)
	if err != nil {
		return "", err
	}
	return sectionPath(dir, key), nil
}

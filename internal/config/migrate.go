package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/tidwall/gjson"
	"github.com/SpherePrime/CLI/vendordeps/tidwall/sjson"
	"github.com/SpherePrime/CLI/vendordeps/x/text/language"
)

// schemaKey is the editor-facing pointer kept in the catch-all file so every
// Prime config file can still be validated against the generated schema.
const schemaKey = "$schema"

// migrateToSectionFiles distributes the pre-split single config files across
// the per-section JSON files in the user config directory, then clears each
// source so it stops shadowing the new layout.
//
// It runs before every load and is safe to repeat: a source is only processed
// while it still holds keys, and processing one empties it.
func migrateToSectionFiles() {
	dest := GlobalConfigDir()
	if dest == "" {
		return
	}

	// Lowest priority first so the machine-owned data file, which used to win,
	// overwrites whatever the hand-written global file carried. Both are
	// applied over existing section content: a legacy file is emptied by its
	// own migration, so what is in it now is newer than any section file.
	sources := []migrationSource{
		{path: legacyConfigPath(dest), dest: dest, keepAnchor: true},
		{path: legacyConfigPath(GlobalDataDir()), dest: dest, keepAnchor: false},
	}

	seen := make(map[string]bool, len(sources))
	for _, src := range sources {
		// A custom data directory can be the config directory, and tests point
		// both at one folder. Processing the same file twice would move it
		// aside before its own keys are all in place.
		if seen[src.path] || !fileHasContent(src.path) {
			continue
		}
		seen[src.path] = true

		if src.keepAnchor {
			migrateGlobalFileToSections(src.path, src.dest)
			continue
		}
		migrateLegacyToSections(src.path, src.dest)
	}
}

// voiceLanguageAuto is the value that hands the language choice to the engine
// rather than naming one.
const voiceLanguageAuto = "auto"

// supersededByDetection are the language codes people reach for that the Web
// Speech endpoint obeys instead of refusing, while meaning something the user
// never asked for: it reads "eng" as English and answers in English, so a value
// like this pins dictation to a language the user may not even speak, which
// from the outside looks exactly like a microphone that stopped working.
//
// Only the two languages the default search already covers are listed. A legacy
// spelling of a language the search would not find, such as "ara" for Arabic,
// is left alone: dropping it would replace a right answer with two wrong ones.
var supersededByDetection = map[string]bool{
	"eng": true,
	"rus": true,
}

// usableLanguageTag reports whether a configured dictation language can be read
// as a language tag, and can be trusted to name the language the user speaks.
//
// x/text is not enough on its own here: it reads "eng" as en and "en_US" as
// en-US without complaint, and accepting either would pin dictation to English
// for a user who wrote something else entirely. The shape of the tag is what
// tells a tag apart from a name or from a separator the endpoint never sees, so
// the primary subtag has to be letters, with x/text left to judge whether the
// code is a real one.
func usableLanguageTag(tag string) bool {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" || tag == voiceLanguageAuto {
		return true // Unset and "auto" both mean "let the engine choose".
	}
	primary, _, _ := strings.Cut(tag, "-")
	if len(primary) < 2 || len(primary) > 3 || !isAlphaSubtag(primary) {
		return false
	}
	if supersededByDetection[primary] {
		return false
	}
	_, err := language.Parse(tag)
	return err == nil
}

// isAlphaSubtag reports whether subtag is two or three ASCII letters.
func isAlphaSubtag(subtag string) bool {
	for _, r := range subtag {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// migrateVoiceLanguage rewrites options.voice.language to "auto" when it holds
// a value no language tag can express, so dictation picks a language instead of
// being silently pinned to the wrong one.
//
// It runs before every load and is safe to repeat. The value it writes is one
// usableLanguageTag accepts, so a second run leaves it alone, and a value the
// user did mean is never touched: a deliberate language is not Prime's to undo,
// which is why the rule is about unreadable values and not about every
// configured language.
func migrateVoiceLanguage() {
	const key = "options.voice.language"
	section := sectionPath(GlobalConfigDir(), key)

	// The setting can sit in the catch-all file or, on an already split
	// config, in options.json, so every global file has to be looked at.
	var shadowing []string
	var offending string
	inSection := false
	seen := make(map[string]bool)
	for _, dir := range []string{GlobalConfigDir(), GlobalDataDir()} {
		for _, path := range configFilesForDir(dir) {
			if seen[path] {
				continue
			}
			seen[path] = true

			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			value := gjson.Get(string(data), key)
			if !value.Exists() || usableLanguageTag(value.String()) {
				continue
			}
			if offending == "" {
				offending = value.String()
			}
			if path == section {
				inSection = true
				continue // rewritten in place below
			}
			shadowing = append(shadowing, path)
		}
	}
	if !inSection && len(shadowing) == 0 {
		return
	}

	slog.Info("Dictation language is not a language tag, falling back to detection",
		"value", offending, "replacement", voiceLanguageAuto)

	// The section file is what Prime writes, so the replacement goes there. A
	// copy left in the catch-all file would shadow it, exactly the way a stale
	// key shadows the section it belongs to, so those are cleared.
	if data, err := os.ReadFile(section); err == nil {
		updated, err := sjson.Set(string(data), key, voiceLanguageAuto)
		if err == nil && updated != string(data) {
			if err := atomicWriteFile(section, []byte(updated), 0o600); err != nil {
				slog.Warn("Failed to migrate dictation language", "error", err)
			}
		}
	}
	for _, path := range shadowing {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		updated, err := sjson.Delete(string(data), key)
		if err != nil || updated == string(data) {
			continue
		}
		if err := atomicWriteFile(path, []byte(updated), 0o600); err != nil {
			slog.Warn("Failed to write migrated config", "path", path, "error", err)
		}
	}
}

// migrationSource is one legacy single-file config awaiting distribution.
type migrationSource struct {
	path string
	dest string
	// keepAnchor marks the file that shares a directory with the section
	// files: it is trimmed in place instead of being moved aside, because its
	// remaining keys (the schema pointer and anything with no section of its
	// own) still belong there.
	keepAnchor bool
}

// migrateLegacyToSections copies every key of a legacy file into its section
// file and renames the source. The rename is what makes the move safe to
// re-run: a source that is already gone is simply skipped.
func migrateLegacyToSections(path, destDir string) {
	keys, err := topLevelKeys(path)
	if err != nil {
		slog.Warn("Skipping config section migration, unreadable config file", "path", path, "error", err)
		return
	}

	migrated, err := copyKeysToSections(path, keys, destDir)
	if err != nil {
		slog.Warn("Skipping config section migration, failed to write section files", "path", path, "error", err)
		return
	}
	if !migrated {
		return
	}

	if err := moveAside(path); err != nil {
		slog.Warn("Migrated config sections but could not move the old file aside", "path", path, "error", err)
		return
	}
	slog.Info("Migrated config into per-section files", "from", path, "to", destDir, "keys", strings.Join(keys, ", "))
}

// migrateGlobalFileToSections copies a global prime.json into the section files
// that live beside it and reduces the original to its schema pointer. The full
// original is preserved next to it as a backup before it is rewritten.
func migrateGlobalFileToSections(path, destDir string) {
	keys, err := topLevelKeys(path)
	if err != nil {
		slog.Warn("Skipping config section migration, unreadable config file", "path", path, "error", err)
		return
	}

	copiable := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == schemaKey {
			continue
		}
		// A key with no section of its own lives in this very file, so there is
		// nowhere to copy it. Trimming it anyway would delete the value.
		if sectionFileFor(key) == legacyConfigName() {
			continue
		}
		copiable = append(copiable, key)
	}
	if len(copiable) == 0 {
		return
	}

	migrated, err := copyKeysToSections(path, copiable, destDir)
	if err != nil || !migrated {
		if err != nil {
			slog.Warn("Skipping config section migration, failed to write section files", "path", path, "error", err)
		}
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := os.WriteFile(migratedConfigPath(filepath.Dir(path)), data, 0o600); err != nil {
		slog.Warn("Could not write config backup before trimming migrated keys", "path", path, "error", err)
		return
	}

	trimmed := string(data)
	for _, key := range copiable {
		if trimmed, err = sjson.Delete(trimmed, key); err != nil {
			slog.Warn("Could not trim migrated key from config", "path", path, "key", key, "error", err)
			return
		}
	}
	if err := atomicWriteFile(path, []byte(trimmed), 0o600); err != nil {
		slog.Warn("Could not trim migrated keys from config", "path", path, "error", err)
		return
	}
	slog.Info("Migrated config into per-section files", "from", path, "keys", strings.Join(copiable, ", "))
}

// copyKeysToSections writes each of keys from source into the section file that
// owns it, replacing whatever that file held for the key. A legacy file is
// emptied by its own migration, so its contents are the newer statement of the
// value. It reports whether any key was actually written.
func copyKeysToSections(source string, keys []string, destDir string) (bool, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return false, err
	}

	wrote := false
	for _, key := range keys {
		value := gjson.Get(string(data), key)
		if !value.Exists() {
			continue
		}

		// options is the one section whose fields are spread over more than one
		// file, so its children are routed individually. Everything else moves
		// whole.
		if key == "options" {
			children, err := optionFields(value)
			if err != nil {
				return wrote, err
			}
			for optsKey, raw := range children {
				if err := mergeKeyIntoFile(sectionPath(destDir, optsKey), optsKey, raw); err != nil {
					return wrote, err
				}
				wrote = true
			}
			continue
		}

		if err := mergeKeyIntoFile(sectionPath(destDir, key), key, value.Raw); err != nil {
			return wrote, err
		}
		wrote = true
	}
	return wrote, nil
}

// optionFields flattens an options object into "options.<field>" keys paired
// with their raw values, so each field can be routed to its own section file.
func optionFields(value gjson.Result) (map[string]string, error) {
	fields := make(map[string]string, len(value.Map()))
	for field, child := range value.Map() {
		fields["options."+field] = child.Raw
	}
	return fields, nil
}

// mergeKeyIntoFile sets one raw JSON value in a config file.
func mergeKeyIntoFile(path, key, raw string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		existing = []byte("{}")
	}

	updated, err := sjson.SetRaw(string(existing), key, raw)
	if err != nil {
		return fmt.Errorf("set %s in %s: %w", key, path, err)
	}
	return atomicWriteFile(path, []byte(updated), 0o600)
}

// topLevelKeys returns the top-level JSON keys of a config file, in the order
// they appear so migrations stay reproducible.
func topLevelKeys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(doc))
	for key := range doc {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys, nil
}

// moveAside renames a file to its ".bak" sibling, replacing an earlier backup.
func moveAside(path string) error {
	target := path + ".bak"
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(path, target)
}

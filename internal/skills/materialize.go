package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

const materializedStampFile = ".prime-skill-stamp"

var (
	builtinStamps   sync.Map
	materializeLock sync.Mutex
)

// BuiltinSkillName returns the skill name for a builtin SKILL.md location
// such as "prime://skills/prime-designer/SKILL.md".
func BuiltinSkillName(location string) (string, bool) {
	relative := strings.TrimPrefix(location, BuiltinPrefix)
	if relative == location {
		return "", false
	}
	name, file, found := strings.Cut(relative, "/")
	if !found || file != SkillFileName || name == "" {
		return "", false
	}
	return name, true
}

// BuiltinHasResources reports whether a builtin skill ships files besides
// SKILL.md, such as scripts or assets that must exist on disk to be used.
func BuiltinHasResources(name string) bool {
	entries, err := fs.ReadDir(builtinFS, path.Join("builtin", name))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() != SkillFileName {
			return true
		}
	}
	return false
}

// MaterializeBuiltin copies a builtin skill to root/name on disk and returns
// that directory. The copy is refreshed whenever the embedded content
// changes, so an updated binary brings updated scripts and assets.
func MaterializeBuiltin(name, root string) (string, error) {
	source := path.Join("builtin", name)
	stamp, err := builtinStamp(source)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, name)

	materializeLock.Lock()
	defer materializeLock.Unlock()

	if current, err := os.ReadFile(filepath.Join(target, materializedStampFile)); err == nil && string(current) == stamp {
		return target, nil
	}
	err = fs.WalkDir(builtinFS, source, func(entryPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative := strings.TrimPrefix(strings.TrimPrefix(entryPath, source), "/")
		destination := filepath.Join(target, filepath.FromSlash(relative))
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		content, err := builtinFS.ReadFile(entryPath)
		if err != nil {
			return err
		}
		return writeFileAtomic(destination, content)
	})
	if err != nil {
		return "", fmt.Errorf("materialize builtin skill %q: %w", name, err)
	}
	if err := writeFileAtomic(filepath.Join(target, materializedStampFile), []byte(stamp)); err != nil {
		return "", fmt.Errorf("materialize builtin skill %q: %w", name, err)
	}
	return target, nil
}

func builtinStamp(source string) (string, error) {
	if cached, ok := builtinStamps.Load(source); ok {
		return cached.(string), nil
	}
	hash := sha256.New()
	found := false
	err := fs.WalkDir(builtinFS, source, func(entryPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := builtinFS.ReadFile(entryPath)
		if err != nil {
			return err
		}
		found = true
		fmt.Fprintf(hash, "%s\x00%d\x00", entryPath, len(content))
		hash.Write(content)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("read builtin skill %q: %w", source, err)
	}
	if !found {
		return "", fmt.Errorf("builtin skill %q has no files", source)
	}
	stamp := hex.EncodeToString(hash.Sum(nil))
	builtinStamps.Store(source, stamp)
	return stamp, nil
}

func writeFileAtomic(destination string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".prime-skill-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		os.Remove(temporaryName)
		return err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryName)
		return err
	}
	if err := os.Rename(temporaryName, destination); err != nil {
		os.Remove(temporaryName)
		return err
	}
	return nil
}

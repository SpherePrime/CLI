package fsext

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/vendordeps/charlievieth/fastwalk"
)

func ListDirectory(initialPath string, ignorePatterns []string, depth, limit int) ([]string, bool, error) {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("Listing directory", "path", initialPath, "depth", depth, "limit", limit, "ignorePatterns", ignorePatterns)
	}

	if depth == 1 && len(ignorePatterns) == 0 {
		return listDirectoryShallow(initialPath, limit)
	}

	if len(ignorePatterns) == 0 {
		if matches, truncated, ok := listDirectoryFromIndex(initialPath, depth, limit); ok {
			return matches, truncated, nil
		}
	}

	found := csync.NewSlice[string]()
	dl := NewDirectoryLister(initialPath)

	conf := fastwalk.Config{
		Follow:   false,
		ToSlash:  fastwalk.DefaultToSlash(),
		Sort:     fastwalk.SortDirsFirst,
		MaxDepth: depth,
	}

	err := fastwalk.Walk(&conf, initialPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		isDir := d.IsDir()
		if dl.shouldIgnore(path, ignorePatterns, isDir) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		if path != initialPath {
			if isDir {
				path = path + string(filepath.Separator)
			}
			found.Append(path)
		}

		if limit > 0 && found.Len() >= limit {
			return filepath.SkipAll
		}

		return nil
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return nil, false, err
	}

	matches, truncated := truncate(slices.Collect(found.Seq()), limit)
	return matches, truncated || errors.Is(err, filepath.SkipAll), nil
}

func listDirectoryShallow(initialPath string, limit int) ([]string, bool, error) {
	entries, err := os.ReadDir(initialPath)
	if err != nil {
		if info, statErr := os.Stat(initialPath); statErr == nil && !info.IsDir() {
			return nil, false, nil
		}
		return nil, false, err
	}

	dl := NewDirectoryLister(initialPath)
	useToSlash := fastwalk.DefaultToSlash()
	basePrefix, sep := buildOutputPrefix(initialPath, useToSlash)

	var dirs []string
	var files []string
	for _, entry := range entries {
		isDir := entry.IsDir()
		if !isDir && limit > 0 && len(files) > limit {
			continue
		}

		name := entry.Name()
		checkPath := filepath.Join(initialPath, name)
		if dl.shouldIgnore(checkPath, nil, isDir) {
			continue
		}

		outPath := basePrefix + name
		if useToSlash {
			outPath = filepath.ToSlash(outPath)
		}
		if isDir {
			outPath += sep
			dirs = append(dirs, outPath)
			if limit > 0 && len(dirs) > limit {
				break
			}
		} else {
			files = append(files, outPath)
		}
	}

	combined := make([]string, 0, len(dirs)+len(files))
	combined = append(combined, dirs...)
	combined = append(combined, files...)
	matches, truncated := truncate(combined, limit)
	return matches, truncated, nil
}

func listDirectoryFromIndex(initialPath string, depth, limit int) ([]string, bool, bool) {
	entries, slashSubPrefix, ok := defaultWorkspaceIndex.cachedSubtreeEntries(initialPath)
	if !ok {
		entries, ok = IndexEntries(context.Background(), initialPath)
		if !ok {
			return nil, false, false
		}
	}

	useToSlash := fastwalk.DefaultToSlash()
	basePrefix, sep := buildOutputPrefix(initialPath, useToSlash)

	var collected []string
	for i := range entries {
		entry := &entries[i]
		relSlash := entry.SlashPath
		relOut := entry.RelPath
		if slashSubPrefix != "" {
			var hasSub bool
			relSlash, hasSub = strings.CutPrefix(entry.SlashPath, slashSubPrefix)
			if !hasSub {
				continue
			}
			if useToSlash {
				relOut = relSlash
			} else {
				relOut = filepath.FromSlash(relSlash)
			}
		} else if useToSlash {
			relOut = relSlash
		}

		if depth > 0 && strings.Count(relSlash, "/")+1 > depth {
			continue
		}

		outPath := basePrefix + relOut
		if entry.IsDir {
			outPath += sep
		}
		collected = append(collected, outPath)
		if limit > 0 && len(collected) > limit {
			break
		}
	}

	matches, truncated := truncate(collected, limit)
	return matches, truncated, true
}

func buildOutputPrefix(initialPath string, useToSlash bool) (string, string) {
	sep := string(filepath.Separator)
	if useToSlash {
		sep = "/"
	}
	clean := filepath.Clean(initialPath)
	if useToSlash {
		clean = filepath.ToSlash(clean)
	}
	if clean == "." {
		return "", sep
	}
	if strings.HasSuffix(clean, sep) {
		return clean, sep
	}
	return clean + sep, sep
}

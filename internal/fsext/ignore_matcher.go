package fsext

import (
	"cmp"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/home"
	gitconfig "github.com/SpherePrime/CLI/vendordeps/go-git/go-git/v5/config"
	"github.com/SpherePrime/CLI/vendordeps/go-git/go-git/v5/plumbing/format/gitignore"
)

var ignoreFileNames = [2]string{".gitignore", ".primeignore"}

var fastIgnoreDirs = map[string]bool{
	".git":            true,
	".svn":            true,
	".hg":             true,
	".bzr":            true,
	".vscode":         true,
	".idea":           true,
	"node_modules":    true,
	"__pycache__":     true,
	".pytest_cache":   true,
	".cache":          true,
	".tmp":            true,
	".Trash":          true,
	".Spotlight-V100": true,
	".fseventsd":      true,
	".prime":          true,
	"OrbStack":        true,
	".local":          true,
	".share":          true,
}

var commonIgnorePatterns = sync.OnceValue(func() []gitignore.Pattern {
	patterns := []string{
		"*.swp",
		"*.swo",
		"*~",
		".DS_Store",
		"Thumbs.db",
		"target",
		"build",
		"dist",
		"out",
		"bin",
		"obj",
		"*.o",
		"*.so",
		"*.dylib",
		"*.dll",
		"*.exe",
		"*.log",
		"*.tmp",
		"*.temp",
		"*.pyc",
		"*.pyo",
		"vendor",
		"Cargo.lock",
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
	}
	return parsePatterns(patterns, nil)
})

var gitGlobalIgnorePatterns = sync.OnceValue(func() []gitignore.Pattern {
	cfg, err := gitconfig.LoadConfig(gitconfig.GlobalScope)
	if err != nil {
		slog.Debug("Failed to load global git config", "error", err)
		return nil
	}

	excludesFilePath := cmp.Or(
		cfg.Raw.Section("core").Options.Get("excludesfile"),
		filepath.Join(home.Config(), "git", "ignore"),
	)
	excludesFilePath = home.Long(excludesFilePath)

	bts, err := os.ReadFile(excludesFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Debug("Failed to read git global excludes file", "path", excludesFilePath, "error", err)
		}
		return nil
	}

	return parsePatterns(strings.Split(string(bts), "\n"), nil)
})

var primeGlobalIgnorePatterns = sync.OnceValue(func() []gitignore.Pattern {
	name := filepath.Join(home.Config(), "prime", "ignore")
	bts, err := os.ReadFile(name)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Debug("Failed to read prime global ignore file", "path", name, "error", err)
		}
		return nil
	}
	lines := strings.Split(string(bts), "\n")
	return parsePatterns(lines, nil)
})

func parsePatterns(lines []string, domain []string) []gitignore.Pattern {
	var patterns []gitignore.Pattern
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, domain))
	}
	return patterns
}

type matcherEntry struct {
	patterns []gitignore.Pattern
	matcher  gitignore.Matcher
}

type directoryLister struct {
	hasIgnoreFile    *csync.Map[string, bool]
	dirPatterns      *csync.Map[string, []gitignore.Pattern]
	combinedMatchers *csync.Map[string, *matcherEntry]
	rootPath         string
	rootPrefix       string
}

func NewDirectoryLister(rootPath string) *directoryLister {
	cleanRoot := filepath.Clean(rootPath)
	rootPrefix := cleanRoot
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}
	return &directoryLister{
		rootPath:         cleanRoot,
		rootPrefix:       rootPrefix,
		hasIgnoreFile:    csync.NewMap[string, bool](),
		dirPatterns:      csync.NewMap[string, []gitignore.Pattern](),
		combinedMatchers: csync.NewMap[string, *matcherEntry](),
	}
}

func pathToComponents(path string) []string {
	if path == "" || path == "." {
		return nil
	}
	return appendPathComponents(nil, path)
}

func appendPathComponents(dst []string, path string) []string {
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' || (filepath.Separator != '/' && path[i] == filepath.Separator) {
			if i > start {
				dst = append(dst, path[start:i])
			}
			start = i + 1
		}
	}
	return dst
}

func (dl *directoryLister) relPath(path string) string {
	if path == dl.rootPath {
		return "."
	}
	if rest, ok := strings.CutPrefix(path, dl.rootPrefix); ok {
		return rest
	}
	rel, err := filepath.Rel(dl.rootPath, path)
	if err != nil {
		return path
	}
	return rel
}

func (dl *directoryLister) isWithinRoot(dir string) bool {
	if dir == dl.rootPath {
		return true
	}
	if dl.rootPath == "." {
		return !filepath.IsAbs(dir) && dir != ".." && !strings.HasPrefix(dir, ".."+string(filepath.Separator))
	}
	return strings.HasPrefix(dir, dl.rootPrefix)
}

func (dl *directoryLister) hasDirIgnoreFiles(dir string) bool {
	return dl.hasIgnoreFile.GetOrSetCoalesced(dir, func() bool {
		for _, ignoreFile := range ignoreFileNames {
			info, err := os.Stat(filepath.Join(dir, ignoreFile))
			if err == nil && !info.IsDir() && info.Size() > 0 {
				return true
			}
		}
		return false
	})
}

func (dl *directoryLister) getDirPatterns(dir string) []gitignore.Pattern {
	if !dl.hasDirIgnoreFiles(dir) {
		return nil
	}
	return dl.dirPatterns.GetOrSetCoalesced(dir, func() []gitignore.Pattern {
		var domain []string
		if relPath := dl.relPath(dir); relPath != "" && relPath != "." {
			domain = pathToComponents(relPath)
		}

		var allPatterns []gitignore.Pattern
		for _, ignoreFile := range ignoreFileNames {
			ignPath := filepath.Join(dir, ignoreFile)
			if content, err := os.ReadFile(ignPath); err == nil {
				lines := strings.Split(string(content), "\n")
				allPatterns = append(allPatterns, parsePatterns(lines, domain)...)
			}
		}
		return allPatterns
	})
}

func (dl *directoryLister) getCombinedMatcherEntry(dir string) *matcherEntry {
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) && filepath.IsAbs(dl.rootPath) {
		dir = filepath.Clean(filepath.Join(dl.rootPath, dir))
	}
	if !dl.isWithinRoot(dir) {
		dir = dl.rootPath
	}

	return dl.combinedMatchers.GetOrSetCoalesced(dir, func() *matcherEntry {
		if dir == dl.rootPath {
			common := commonIgnorePatterns()
			gitGlobal := gitGlobalIgnorePatterns()
			primeGlobal := primeGlobalIgnorePatterns()
			rootLocal := dl.getDirPatterns(dl.rootPath)

			allPatterns := make([]gitignore.Pattern, 0, len(common)+len(gitGlobal)+len(primeGlobal)+len(rootLocal))
			allPatterns = append(allPatterns, common...)
			allPatterns = append(allPatterns, gitGlobal...)
			allPatterns = append(allPatterns, primeGlobal...)
			allPatterns = append(allPatterns, rootLocal...)
			return &matcherEntry{
				patterns: allPatterns,
				matcher:  gitignore.NewMatcher(allPatterns),
			}
		}

		parentDir := filepath.Dir(dir)
		parent := dl.getCombinedMatcherEntry(parentDir)
		local := dl.getDirPatterns(dir)
		if len(local) == 0 {
			return parent
		}

		combined := make([]gitignore.Pattern, 0, len(parent.patterns)+len(local))
		combined = append(combined, parent.patterns...)
		combined = append(combined, local...)
		return &matcherEntry{
			patterns: combined,
			matcher:  gitignore.NewMatcher(combined),
		}
	})
}

func (dl *directoryLister) shouldIgnore(path string, ignorePatterns []string, isDir bool) bool {
	base := filepath.Base(path)

	if isDir && fastIgnoreDirs[base] {
		return true
	}

	for _, pattern := range ignorePatterns {
		if matched, err := filepath.Match(pattern, base); err == nil && matched {
			return true
		}
	}

	if path == dl.rootPath {
		return false
	}

	relPath := dl.relPath(path)
	if relPath == "" || relPath == "." {
		return false
	}

	var stackBuf [16]string
	pathComponents := appendPathComponents(stackBuf[:0], relPath)
	if len(pathComponents) == 0 {
		return false
	}

	parentDir := filepath.Dir(path)
	matcher := dl.getCombinedMatcherEntry(parentDir).matcher

	if matcher.Match(pathComponents, isDir) {
		if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
			slog.Debug("Ignoring path", "path", relPath)
		}
		return true
	}

	return false
}

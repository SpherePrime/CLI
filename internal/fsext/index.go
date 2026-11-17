package fsext

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/charlievieth/fastwalk"
	"github.com/SpherePrime/CLI/vendordeps/fsnotify/fsnotify"
)

const (
	defaultMaxIndexedRoots   = 8
	defaultMaxEntriesPerRoot = 200_000
	defaultFallbackTTL       = 10 * time.Second
	defaultMaxWatchedDirs    = 512
)

var (
	errBuildCancelled = errors.New("workspace index build cancelled")
	errBuildOverCap   = errors.New("workspace index entry cap exceeded")
)

type IndexEntry struct {
	Path      string
	RelPath   string
	SlashPath string
	Name      string
	IsDir     bool
	Size      int64
	ModTime   time.Time
}

type WorkspaceIndexConfig struct {
	MaxRoots       int
	MaxEntries     int
	FallbackTTL    time.Duration
	MaxWatchedDirs int
	WatchEnabled   bool
}

type indexedRoot struct {
	root            string
	entries         []IndexEntry
	builtAt         time.Time
	lastAccess      uint64
	watcher         *fsnotify.Watcher
	watcherActive   bool
	watcherComplete bool
	closeOnce       sync.Once
}

func (r *indexedRoot) closeWatcher() {
	r.closeOnce.Do(func() {
		if r.watcher != nil {
			_ = r.watcher.Close()
		}
	})
}

type inflightIndexBuild struct {
	done        chan struct{}
	entries     []IndexEntry
	ok          bool
	invalidated atomic.Bool
}

type WorkspaceIndex struct {
	mu             sync.Mutex
	closed         bool
	roots          map[string]*indexedRoot
	inflight       map[string]*inflightIndexBuild
	accessSeq      uint64
	maxRoots       int
	maxEntries     int
	fallbackTTL    time.Duration
	maxWatchedDirs int
	watchEnabled   bool
}

var defaultWorkspaceIndex = NewWorkspaceIndex(WorkspaceIndexConfig{
	MaxRoots:       defaultMaxIndexedRoots,
	MaxEntries:     defaultMaxEntriesPerRoot,
	FallbackTTL:    defaultFallbackTTL,
	MaxWatchedDirs: defaultMaxWatchedDirs,
	WatchEnabled:   !testing.Testing(),
})

func NewWorkspaceIndex(cfg WorkspaceIndexConfig) *WorkspaceIndex {
	if cfg.MaxRoots <= 0 {
		cfg.MaxRoots = defaultMaxIndexedRoots
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = defaultMaxEntriesPerRoot
	}
	if cfg.FallbackTTL <= 0 {
		cfg.FallbackTTL = defaultFallbackTTL
	}
	if cfg.MaxWatchedDirs <= 0 {
		cfg.MaxWatchedDirs = defaultMaxWatchedDirs
	}
	return &WorkspaceIndex{
		roots:          make(map[string]*indexedRoot),
		inflight:       make(map[string]*inflightIndexBuild),
		maxRoots:       cfg.MaxRoots,
		maxEntries:     cfg.MaxEntries,
		fallbackTTL:    cfg.FallbackTTL,
		maxWatchedDirs: cfg.MaxWatchedDirs,
		watchEnabled:   cfg.WatchEnabled,
	}
}

func IndexEntries(ctx context.Context, rootPath string) ([]IndexEntry, bool) {
	return defaultWorkspaceIndex.IndexEntries(ctx, rootPath)
}

func InvalidateIndex(path string) {
	defaultWorkspaceIndex.InvalidateIndex(path)
}

func InvalidateRoot(rootPath string) {
	defaultWorkspaceIndex.InvalidateRoot(rootPath)
}

func (idx *WorkspaceIndex) Close() {
	idx.mu.Lock()
	idx.closed = true
	toClose := make([]*indexedRoot, 0, len(idx.roots))
	for k, entry := range idx.roots {
		toClose = append(toClose, entry)
		delete(idx.roots, k)
	}
	for _, build := range idx.inflight {
		build.invalidated.Store(true)
	}
	idx.mu.Unlock()

	for _, entry := range toClose {
		entry.closeWatcher()
	}
}

func (idx *WorkspaceIndex) IndexEntries(ctx context.Context, rootPath string) ([]IndexEntry, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return nil, false
	}

	absRoot, ok := cleanAbsRoot(rootPath)
	if !ok {
		return nil, false
	}

	for {
		var expired *indexedRoot

		idx.mu.Lock()
		if idx.closed {
			idx.mu.Unlock()
			return nil, false
		}
		if entry, exists := idx.roots[absRoot]; exists {
			if !idx.isEntryExpired(entry) {
				idx.accessSeq++
				entry.lastAccess = idx.accessSeq
				entries := entry.entries
				idx.mu.Unlock()
				return entries, true
			}
			delete(idx.roots, absRoot)
			expired = entry
		}

		if build, active := idx.inflight[absRoot]; active {
			idx.mu.Unlock()
			if expired != nil {
				expired.closeWatcher()
			}
			select {
			case <-build.done:
				if build.invalidated.Load() {
					continue
				}
				return build.entries, build.ok
			case <-ctx.Done():
				return nil, false
			}
		}

		build := &inflightIndexBuild{
			done: make(chan struct{}),
		}
		idx.inflight[absRoot] = build
		idx.mu.Unlock()

		if expired != nil {
			expired.closeWatcher()
		}

		entries, dirsToWatch, ok := idx.buildRootIndex(ctx, absRoot)
		idx.mu.Lock()
		if !ok || idx.closed || build.invalidated.Load() {
			if idx.inflight[absRoot] == build {
				delete(idx.inflight, absRoot)
			}
			idx.mu.Unlock()
			close(build.done)
			return nil, false
		}
		idx.mu.Unlock()

		entry := &indexedRoot{
			root:    absRoot,
			entries: entries,
			builtAt: time.Now(),
		}
		watcher := idx.prepareWatcher(absRoot, dirsToWatch, entry)

		var evicted *indexedRoot
		idx.mu.Lock()
		if idx.closed || build.invalidated.Load() {
			if idx.inflight[absRoot] == build {
				delete(idx.inflight, absRoot)
			}
			idx.mu.Unlock()
			if watcher != nil {
				entry.closeWatcher()
			}
			close(build.done)
			return nil, false
		}

		if len(idx.roots) >= idx.maxRoots {
			if _, alreadyPresent := idx.roots[absRoot]; !alreadyPresent {
				evicted = idx.evictLRULocked()
			}
		}
		idx.accessSeq++
		entry.lastAccess = idx.accessSeq
		idx.roots[absRoot] = entry
		build.entries = entries
		build.ok = true
		if idx.inflight[absRoot] == build {
			delete(idx.inflight, absRoot)
		}
		idx.mu.Unlock()

		if evicted != nil {
			evicted.closeWatcher()
		}
		if watcher != nil {
			go idx.watchLoop(absRoot, entry, watcher)
		}
		close(build.done)
		return entries, true
	}
}

func (idx *WorkspaceIndex) cachedSubtreeEntries(targetPath string) ([]IndexEntry, string, bool) {
	absTarget, ok := cleanAbsRoot(targetPath)
	if !ok {
		return nil, "", false
	}

	var expired []*indexedRoot
	var matchedEntries []IndexEntry
	var slashSubPrefix string
	var found bool

	idx.mu.Lock()
	if idx.closed {
		idx.mu.Unlock()
		return nil, "", false
	}
	if entry, exists := idx.roots[absTarget]; exists {
		if idx.isEntryExpired(entry) {
			delete(idx.roots, absTarget)
			expired = append(expired, entry)
		} else {
			idx.accessSeq++
			entry.lastAccess = idx.accessSeq
			matchedEntries = entry.entries
			found = true
		}
	}

	if !found && !fastIgnoreDirs[filepath.Base(absTarget)] {
		for root, entry := range idx.roots {
			if idx.isEntryExpired(entry) {
				delete(idx.roots, root)
				expired = append(expired, entry)
				continue
			}
			if !isSubpath(absTarget, root) {
				continue
			}
			rel, err := filepath.Rel(root, absTarget)
			if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
				continue
			}
			relSlash := filepath.ToSlash(rel)
			if !containsIndexedDir(entry.entries, relSlash) {
				continue
			}
			idx.accessSeq++
			entry.lastAccess = idx.accessSeq
			matchedEntries = entry.entries
			slashSubPrefix = relSlash + "/"
			found = true
			break
		}
	}
	idx.mu.Unlock()

	for _, e := range expired {
		e.closeWatcher()
	}
	return matchedEntries, slashSubPrefix, found
}

func containsIndexedDir(entries []IndexEntry, slashPath string) bool {
	for i := range entries {
		if entries[i].IsDir && entries[i].SlashPath == slashPath {
			return true
		}
	}
	return false
}

func (idx *WorkspaceIndex) InvalidateRoot(rootPath string) {
	absRoot, ok := cleanAbsRoot(rootPath)
	if !ok {
		return
	}

	idx.mu.Lock()
	entry := idx.roots[absRoot]
	if entry != nil {
		delete(idx.roots, absRoot)
	}
	if build, active := idx.inflight[absRoot]; active {
		build.invalidated.Store(true)
	}
	idx.mu.Unlock()

	if entry != nil {
		entry.closeWatcher()
	}
}

func (idx *WorkspaceIndex) InvalidateIndex(path string) {
	absPath, ok := cleanAbsRoot(path)
	if !ok {
		return
	}

	var toClose []*indexedRoot

	idx.mu.Lock()
	for root, entry := range idx.roots {
		if pathsOverlap(absPath, root) {
			delete(idx.roots, root)
			toClose = append(toClose, entry)
		}
	}
	for root, build := range idx.inflight {
		if pathsOverlap(absPath, root) {
			build.invalidated.Store(true)
		}
	}
	idx.mu.Unlock()

	for _, entry := range toClose {
		entry.closeWatcher()
	}
}

func (idx *WorkspaceIndex) invalidateEntry(absRoot string, expected *indexedRoot) {
	idx.mu.Lock()
	current := idx.roots[absRoot]
	if current == expected {
		delete(idx.roots, absRoot)
	} else {
		expected = nil
	}
	idx.mu.Unlock()

	if expected != nil {
		expected.closeWatcher()
	}
}

func (idx *WorkspaceIndex) isEntryExpired(entry *indexedRoot) bool {
	if idx.fallbackTTL <= 0 {
		return false
	}
	if !entry.watcherActive || !entry.watcherComplete {
		return time.Since(entry.builtAt) > idx.fallbackTTL
	}
	return false
}

func (idx *WorkspaceIndex) evictLRULocked() *indexedRoot {
	var oldestKey string
	var oldestEntry *indexedRoot
	for k, e := range idx.roots {
		if oldestEntry == nil || e.lastAccess < oldestEntry.lastAccess {
			oldestKey = k
			oldestEntry = e
		}
	}
	if oldestEntry != nil {
		delete(idx.roots, oldestKey)
	}
	return oldestEntry
}

func (idx *WorkspaceIndex) buildRootIndex(ctx context.Context, absRoot string) ([]IndexEntry, []string, bool) {
	info, err := os.Stat(absRoot)
	if err != nil || !info.IsDir() {
		return nil, nil, false
	}

	dl := NewDirectoryLister(absRoot)
	var collectMu sync.Mutex
	var entries []IndexEntry
	var dirsToWatch []string
	var overCap atomic.Bool

	conf := fastwalk.Config{
		Follow:  false,
		ToSlash: false,
		Sort:    fastwalk.SortDirsFirst,
	}

	err = fastwalk.Walk(&conf, absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return errBuildCancelled
		}
		if overCap.Load() {
			return errBuildOverCap
		}
		if walkErr != nil {
			if errors.Is(walkErr, errBuildOverCap) || errors.Is(walkErr, errBuildCancelled) {
				return walkErr
			}
			return nil
		}
		if path == absRoot {
			return nil
		}

		isDir := d.IsDir()
		if dl.shouldIgnore(path, nil, isDir) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		entInfo, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		relPath := dl.relPath(path)
		slashPath := filepath.ToSlash(relPath)

		collectMu.Lock()
		if len(entries) >= idx.maxEntries {
			overCap.Store(true)
			collectMu.Unlock()
			return errBuildOverCap
		}
		entries = append(entries, IndexEntry{
			Path:      path,
			RelPath:   relPath,
			SlashPath: slashPath,
			Name:      d.Name(),
			IsDir:     isDir,
			Size:      entInfo.Size(),
			ModTime:   entInfo.ModTime(),
		})
		if isDir && idx.watchEnabled && len(dirsToWatch) < idx.maxWatchedDirs {
			dirsToWatch = append(dirsToWatch, path)
		}
		collectMu.Unlock()

		return nil
	})
	if err != nil || ctx.Err() != nil || overCap.Load() {
		return nil, nil, false
	}

	return entries, dirsToWatch, true
}

func (idx *WorkspaceIndex) prepareWatcher(absRoot string, dirs []string, entry *indexedRoot) *fsnotify.Watcher {
	if !idx.watchEnabled {
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil
	}
	if err := watcher.Add(absRoot); err != nil {
		_ = watcher.Close()
		return nil
	}

	complete := true
	watched := 1
	for _, dir := range dirs {
		if watched >= idx.maxWatchedDirs {
			complete = false
			break
		}
		if err := watcher.Add(dir); err != nil {
			complete = false
			continue
		}
		watched++
	}

	entry.watcher = watcher
	entry.watcherActive = true
	entry.watcherComplete = complete
	return watcher
}

func (idx *WorkspaceIndex) watchLoop(absRoot string, entry *indexedRoot, watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create | fsnotify.Remove | fsnotify.Rename | fsnotify.Write) {
				idx.invalidateEntry(absRoot, entry)
				return
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func cleanAbsRoot(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	return filepath.Clean(abs), true
}

func pathsOverlap(a, b string) bool {
	return a == b || isSubpath(a, b) || isSubpath(b, a)
}

func isSubpath(path, root string) bool {
	if path == root {
		return false
	}
	if strings.HasSuffix(root, string(filepath.Separator)) {
		return strings.HasPrefix(path, root)
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

package voice

import (
	"fmt"
	"sync"
	"time"
)

type voskModelEntry struct {
	handle     uintptr
	users      int
	discard    bool
	timer      *time.Timer
	generation uint64
}

type voskModelCache struct {
	mu        sync.Mutex
	models    map[string]*voskModelEntry
	create    func(string) uintptr
	free      func(uintptr)
	idleDelay time.Duration
}

func newVoskModelCache(create func(string) uintptr, free func(uintptr), idleDelay time.Duration) *voskModelCache {
	return &voskModelCache{models: map[string]*voskModelEntry{}, create: create, free: free, idleDelay: idleDelay}
}

func (cache *voskModelCache) removeLocked(path string, entry *voskModelEntry) {
	if entry.timer != nil {
		entry.timer.Stop()
	}
	delete(cache.models, path)
	cache.free(entry.handle)
}

func (cache *voskModelCache) acquire(path string) (uintptr, func(), error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	for otherPath, entry := range cache.models {
		if otherPath != path && entry.users == 0 {
			cache.removeLocked(otherPath, entry)
		}
	}
	entry := cache.models[path]
	if entry == nil {
		handle := cache.create(path)
		if handle == 0 {
			return 0, nil, fmt.Errorf("%w: vosk model at %s could not be loaded", ErrTranscribeFailed, path)
		}
		entry = &voskModelEntry{handle: handle}
		cache.models[path] = entry
	}
	if entry.timer != nil {
		entry.timer.Stop()
	}
	entry.generation++
	entry.users++
	var released sync.Once
	release := func() {
		released.Do(func() {
			cache.mu.Lock()
			defer cache.mu.Unlock()
			entry.users--
			if entry.users != 0 {
				return
			}
			if entry.discard {
				cache.removeLocked(path, entry)
				return
			}
			for otherPath, other := range cache.models {
				if other != entry && other.users == 0 {
					cache.removeLocked(otherPath, other)
				}
			}
			entry.generation++
			generation := entry.generation
			entry.timer = time.AfterFunc(cache.idleDelay, func() {
				cache.mu.Lock()
				defer cache.mu.Unlock()
				if cache.models[path] == entry && entry.users == 0 && entry.generation == generation {
					cache.removeLocked(path, entry)
				}
			})
		})
	}
	return entry.handle, release, nil
}

func (cache *voskModelCache) remove(path string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	entry := cache.models[path]
	if entry == nil {
		return
	}
	if entry.users > 0 {
		entry.discard = true
		return
	}
	cache.removeLocked(path, entry)
}

package voice

import (
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestVoskModelCacheReusesActiveModelsAndReleasesIdleMemory(t *testing.T) {
	var mu sync.Mutex
	var loaded, freed []uintptr
	cache := newVoskModelCache(func(path string) uintptr {
		mu.Lock()
		defer mu.Unlock()
		handle := uintptr(len(loaded) + 1)
		loaded = append(loaded, handle)
		return handle
	}, func(handle uintptr) {
		mu.Lock()
		defer mu.Unlock()
		freed = append(freed, handle)
	}, 30*time.Millisecond)
	a, releaseA, err := cache.acquire("russian")
	require.NoError(t, err)
	a2, releaseA2, err := cache.acquire("russian")
	require.NoError(t, err)
	require.Equal(t, a, a2)
	b, releaseB, err := cache.acquire("english")
	require.NoError(t, err)
	releaseA()
	releaseA2()
	_, releaseB2, err := cache.acquire("english")
	require.NoError(t, err)
	mu.Lock()
	require.Equal(t, []uintptr{a}, freed)
	mu.Unlock()
	releaseB()
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	require.Equal(t, []uintptr{a}, freed)
	mu.Unlock()
	releaseB2()
	releaseB2()
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(freed) == 2 && freed[1] == b
	}, time.Second, 5*time.Millisecond)
	_, releaseReloaded, err := cache.acquire("english")
	require.NoError(t, err)
	cache.remove("english")
	mu.Lock()
	require.Len(t, freed, 2)
	mu.Unlock()
	releaseReloaded()
	mu.Lock()
	require.Len(t, loaded, 3)
	require.Len(t, freed, 3)
	mu.Unlock()
}

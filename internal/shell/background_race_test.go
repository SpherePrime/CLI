package shell

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestBackgroundShellCancelsContextOnNormalCompletion(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	bgShell, err := manager.Start(t.Context(), t.TempDir(), nil, "echo done", "normal-completion")
	require.NoError(t, err)

	require.True(t, bgShell.WaitContext(t.Context()))
	require.ErrorIs(t, bgShell.ctx.Err(), context.Canceled)
	_ = manager.Remove(bgShell.ID)
}

func TestBackgroundShellManagerEnforcesMaxBackgroundJobsConcurrently(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	for i := range MaxBackgroundJobs - 2 {
		id := string(rune('A'+i/26)) + string(rune('a'+i%26))
		manager.shells.Set(id, &BackgroundShell{
			ID:     id,
			done:   make(chan struct{}),
			cancel: func() {},
		})
	}

	workingDir := t.TempDir()
	var succeeded atomic.Int32
	var started []*BackgroundShell
	var mu sync.Mutex
	var wg sync.WaitGroup

	for range 16 {
		wg.Go(func() {
			bgShell, err := manager.Start(t.Context(), workingDir, nil, "sleep 5", "concurrent-limit")
			if err == nil {
				succeeded.Add(1)
				mu.Lock()
				started = append(started, bgShell)
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	require.LessOrEqual(t, int(succeeded.Load()), 2)

	for _, s := range started {
		_ = manager.Kill(s.ID)
		require.Eventually(t, s.IsDone, 5*time.Second, 25*time.Millisecond)
	}
}

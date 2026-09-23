//go:build windows

package shell

import (
	"context"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// Cancelling a real subprocess must terminate the whole process tree and
// release the inherited pipes, so the background shell reports done shortly
// after Kill. Before the Windows exec handler killed process trees, the
// grandchild kept the stdout handle open and cmd.Wait blocked forever,
// which surfaced as job_output and job_kill hanging.
func TestKillTerminatesWindowsProcessTree(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	bgShell, err := manager.Start(
		context.Background(),
		t.TempDir(),
		nil,
		`cmd /c "ping -n 60 127.0.0.1 >nul"`,
		"",
	)
	require.NoError(t, err)

	// Give the child time to start and spawn its own grandchild.
	time.Sleep(700 * time.Millisecond)

	require.NoError(t, manager.Kill(bgShell.ID))

	select {
	case <-bgShell.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("background shell did not exit after kill: process tree was not terminated")
	}

	require.True(t, bgShell.IsDone())
}

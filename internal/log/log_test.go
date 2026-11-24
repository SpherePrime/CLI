package log

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDesktopLogDirUsesDotPrimeLogs(t *testing.T) {
	t.Setenv("PRIME_DESKTOP_LOG_DIR", "")
	dir := DefaultDesktopLogDir()
	suffix := filepath.Join(".prime", "logs")
	if !strings.HasSuffix(dir, suffix) {
		t.Fatalf("expected desktop log dir %q to end with %q", dir, suffix)
	}
}

func TestDefaultDesktopLogDirHonorsOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-logs")
	t.Setenv("PRIME_DESKTOP_LOG_DIR", custom)
	if got := DefaultDesktopLogDir(); got != custom {
		t.Fatalf("expected %q, got %q", custom, got)
	}
	if got := DefaultDesktopLogFile(); got != filepath.Join(custom, "desktop-engine.log") {
		t.Fatalf("unexpected log file path: %q", got)
	}
}

func TestRecoverPanicRunsCleanupEvenWhenLogFileFails(t *testing.T) {
	prev := activeLogFile.Load()
	missingDir := filepath.Join(t.TempDir(), "nonexistent-dir", "child", "engine.log")
	activeLogFile.Store(missingDir)
	t.Cleanup(func() {
		if prev != nil {
			activeLogFile.Store(prev)
		} else {
			activeLogFile.Store("")
		}
	})

	cleaned := false
	func() {
		defer RecoverPanic("test", func() { cleaned = true })
		panic("boom")
	}()
	if !cleaned {
		t.Fatal("expected cleanup to run even when panic log creation fails")
	}
}

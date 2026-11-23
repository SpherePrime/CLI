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

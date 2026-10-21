//go:build windows

package lsp

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/x/sys/windows"
)

func TestLanguageServerTreeHelper(t *testing.T) {
	switch os.Getenv("POWERNAP_TREE_HELPER") {
	case "server":
		child := exec.Command(os.Args[0], "-test.run=^TestLanguageServerTreeHelper$")
		child.Env = append(os.Environ(), "POWERNAP_TREE_HELPER=sleep")
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		_ = os.WriteFile(os.Getenv("POWERNAP_TREE_PID_FILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o600)
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	case "sleep":
		time.Sleep(2 * time.Minute)
		os.Exit(0)
	default:
		t.Skip("helper process")
	}
}

func exitedWithin(pid int, timeout time.Duration) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true
	}
	defer windows.CloseHandle(handle)
	event, _ := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	return event == windows.WAIT_OBJECT_0
}

func TestServerProcessCloseReapsDescendants(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	stream, err := startServerProcess(t.Context(), ClientConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=^TestLanguageServerTreeHelper$"},
		Environment: map[string]string{
			"POWERNAP_TREE_HELPER":   "server",
			"POWERNAP_TREE_PID_FILE": pidFile,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var grandchild int
	deadline := time.Now().Add(10 * time.Second)
	for grandchild == 0 && time.Now().Before(deadline) {
		if data, err := os.ReadFile(pidFile); err == nil {
			grandchild, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		time.Sleep(50 * time.Millisecond)
	}
	if grandchild == 0 {
		t.Fatal("helper did not report its child")
	}
	t.Cleanup(func() {
		if process, err := os.FindProcess(grandchild); err == nil {
			_ = process.Kill()
		}
	})
	_ = stream.Close()
	if !exitedWithin(grandchild, 10*time.Second) {
		t.Fatal("closing a language server must stop its child processes")
	}
}

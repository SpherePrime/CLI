//go:build windows

package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/modelcontextprotocol/go-sdk/mcp"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"github.com/SpherePrime/CLI/vendordeps/x/sys/windows"
)

func TestMCPProcessTreeHelper(t *testing.T) {
	switch os.Getenv("PRIME_TREE_HELPER") {
	case "server":
		child := exec.Command(os.Args[0], "-test.run=^TestMCPProcessTreeHelper$")
		child.Env = append(os.Environ(), "PRIME_TREE_HELPER=sleep")
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		_ = os.WriteFile(os.Getenv("PRIME_TREE_PID_FILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o600)
		server := mcp.NewServer(&mcp.Implementation{Name: "tree", Version: "1"}, nil)
		_ = server.Run(context.Background(), &mcp.StdioTransport{})
		os.Exit(0)
	case "sleep":
		time.Sleep(2 * time.Minute)
		os.Exit(0)
	default:
		t.Skip("helper process")
	}
}

func processExited(pid int, timeout time.Duration) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true
	}
	defer windows.CloseHandle(handle)
	event, _ := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	return event == windows.WAIT_OBJECT_0
}

func TestStdioSessionCloseReapsDescendants(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	m := config.MCPConfig{
		Type:    config.MCPStdio,
		Command: os.Args[0],
		Args:    []string{"-test.run=^TestMCPProcessTreeHelper$"},
		Env: map[string]string{
			"PRIME_TREE_HELPER":   "server",
			"PRIME_TREE_PID_FILE": pidFile,
		},
		Timeout: 30,
	}
	session, err := createSession(t.Context(), nil, "tree-test", m, shellResolverWithPath(t, nil), false)
	require.NoError(t, err)
	var grandchild int
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		grandchild, err = strconv.Atoi(strings.TrimSpace(string(data)))
		return err == nil
	}, 10*time.Second, 50*time.Millisecond)
	t.Cleanup(func() {
		if process, err := os.FindProcess(grandchild); err == nil {
			_ = process.Kill()
		}
	})
	require.False(t, processExited(grandchild, 200*time.Millisecond))
	_ = session.Close()
	require.True(t, processExited(grandchild, 10*time.Second), "closing an MCP session must stop the server's child processes")
}

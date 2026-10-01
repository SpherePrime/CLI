package mcp

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestStdioDiagnosticHelperProcess is not a real test. It is the child
// process the stdio-diagnostic tests re-execute: those tests re-run a
// deliberately broken stdio server command and assert that its diagnostic
// text reaches the error.
//
// The child is this very test binary, invoked with PRIME_STDIO_HELPER set.
// Using it instead of "sh -c" keeps the tests honest on every platform: a
// stock Windows box has no "sh" on PATH, and cmd.exe writes UTF-16LE to a
// redirected pipe, so neither could produce the plain diagnostic bytes these
// assertions look for. A re-exec'd Go binary produces them everywhere.
func TestStdioDiagnosticHelperProcess(t *testing.T) {
	text := os.Getenv("PRIME_STDIO_HELPER_TEXT")
	if text == "" {
		t.Skip("helper process; runs only when re-executed by a stdio diagnostic test")
	}
	if os.Getenv("PRIME_STDIO_HELPER_STDERR") == "1" {
		fmt.Fprintln(os.Stderr, text)
	} else {
		fmt.Fprintln(os.Stdout, text)
	}
	// Exit before the testing framework can report a pass, so the child
	// looks like a server that crashed at startup.
	os.Exit(3)
}

// newFailingChildCmd builds the *exec.Cmd for a child that prints text and
// exits with status 3, the shape a misconfigured MCP server has at startup.
func newFailingChildCmd(t *testing.T, text string, toStderr bool) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestStdioDiagnosticHelperProcess$")
	cmd.Env = append(os.Environ(),
		"PRIME_STDIO_HELPER_TEXT="+text,
		fmt.Sprintf("PRIME_STDIO_HELPER_STDERR=%t", toStderr),
	)
	return cmd
}

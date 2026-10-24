package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type wakeCall struct {
	sessionID, jobID, status, output string
}

func slowMCPTool(t *testing.T, release <-chan struct{}) (*Tool, chan wakeCall) {
	t.Helper()
	previousRun, previousAfter := runMCPTool, MCPBackgroundAfter
	t.Cleanup(func() { runMCPTool, MCPBackgroundAfter = previousRun, previousAfter })
	MCPBackgroundAfter = 50 * time.Millisecond
	runMCPTool = func(ctx context.Context, _ *config.ConfigStore, _, _ string, _ string, _ string, _ string) (mcp.ToolResult, error) {
		select {
		case <-release:
			return mcp.ToolResult{Type: "text", Content: "slow result"}, nil
		case <-ctx.Done():
			return mcp.ToolResult{}, ctx.Err()
		}
	}
	wakes := make(chan wakeCall, 1)
	tool := &Tool{
		mcpName:     "fixture",
		tool:        &mcp.Tool{Name: "browser_evaluate"},
		permissions: &mockPermissionService{},
		wake: func(sessionID, jobID, _, status, output string) {
			wakes <- wakeCall{sessionID, jobID, status, output}
		},
	}
	return tool, wakes
}

func mcpJobMetadata(t *testing.T, response fantasy.ToolResponse) MCPJobMetadata {
	t.Helper()
	var metadata MCPJobMetadata
	require.NoError(t, json.Unmarshal([]byte(response.Metadata), &metadata))
	return metadata
}

func TestMCPCallMovesToBackgroundAndWakesOnCompletion(t *testing.T) {
	release := make(chan struct{})
	tool, wakes := slowMCPTool(t, release)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session")
	response, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: tool.Name(), Input: `{"expression":"1"}`})
	require.NoError(t, err)
	require.Contains(t, response.Content, "continues in the background")
	metadata := mcpJobMetadata(t, response)
	require.True(t, metadata.Background)

	status := NewMCPJobTool(nil)
	result, err := status.Run(ctx, fantasy.ToolCall{ID: "status", Name: MCPJobToolName, Input: `{"op":"status","job_id":"` + metadata.JobID + `"}`})
	require.NoError(t, err)
	require.Contains(t, result.Content, "is running for")
	require.Contains(t, result.Content, "Verdict: likely hung")

	close(release)
	select {
	case wake := <-wakes:
		require.Equal(t, "session", wake.sessionID)
		require.Equal(t, metadata.JobID, wake.jobID)
		require.Equal(t, "done", wake.status)
		require.Equal(t, "slow result", wake.output)
	case <-time.After(2 * time.Second):
		t.Fatal("background MCP job did not wake the session")
	}
	result, err = status.Run(ctx, fantasy.ToolCall{ID: "result", Name: MCPJobToolName, Input: `{"op":"result","job_id":"` + metadata.JobID + `"}`})
	require.NoError(t, err)
	require.Equal(t, "slow result", result.Content)
	require.True(t, MCPJobResultSeen(metadata.JobID))
}

func TestMCPJobKillCancelsCallWithoutWake(t *testing.T) {
	tool, wakes := slowMCPTool(t, make(chan struct{}))
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session")
	response, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: tool.Name(), Input: `{}`})
	require.NoError(t, err)
	metadata := mcpJobMetadata(t, response)

	jobs := NewMCPJobTool(nil)
	other := context.WithValue(context.Background(), SessionIDContextKey, "other")
	result, err := jobs.Run(other, fantasy.ToolCall{ID: "foreign", Name: MCPJobToolName, Input: `{"op":"kill","job_id":"` + metadata.JobID + `"}`})
	require.NoError(t, err)
	require.True(t, result.IsError)

	_, err = jobs.Run(ctx, fantasy.ToolCall{ID: "kill", Name: MCPJobToolName, Input: `{"op":"kill","job_id":"` + metadata.JobID + `"}`})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		job, _ := mcpJobs.Get(metadata.JobID)
		status, _, _, _ := job.snapshot()
		return status == "canceled"
	}, 2*time.Second, 10*time.Millisecond)
	select {
	case wake := <-wakes:
		t.Fatalf("killed job woke the session: %+v", wake)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestFastMCPCallReturnsInline(t *testing.T) {
	release := make(chan struct{})
	close(release)
	tool, _ := slowMCPTool(t, release)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session")
	response, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: tool.Name(), Input: `{}`})
	require.NoError(t, err)
	require.Equal(t, "slow result", response.Content)
	require.False(t, strings.Contains(response.Metadata, "background"))
}

package agent

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func toolStep(name, input string) fantasy.StepResult {
	return fantasy.StepResult{Response: fantasy.Response{Content: fantasy.ResponseContent{
		fantasy.ToolCallContent{ToolCallID: name + input, ToolName: name, Input: input},
	}}}
}

func TestPollingSameReadIsDetected(t *testing.T) {
	var steps []fantasy.StepResult
	for range 4 {
		steps = append(steps, toolStep("ls", `{"path":"src"}`), toolStep("view", `{"file_path": "src/app.css"}`))
	}
	require.True(t, hasPollingCalls(steps))
}

func TestReadingDifferentFilesIsNotPolling(t *testing.T) {
	var steps []fantasy.StepResult
	for _, file := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		steps = append(steps, toolStep("view", `{"file_path":"`+file+`"}`))
	}
	require.False(t, hasPollingCalls(steps))
}

func TestRepeatedSleepWaitingIsDetected(t *testing.T) {
	steps := []fantasy.StepResult{
		toolStep("bash", `{"command":"sleep 10 && ls"}`),
		toolStep("view", `{"file_path":"a"}`),
	}
	require.False(t, hasPollingCalls(steps))
	steps = append(steps, toolStep("bash", `{"command":"sleep 15; ls -la"}`))
	require.True(t, hasPollingCalls(steps))
	require.True(t, hasPollingCalls([]fantasy.StepResult{
		toolStep("bash", `{"command":"Start-Sleep -Seconds 20; Get-ChildItem"}`),
		toolStep("bash", `{"command":"Start-Sleep 25"}`),
	}))
}

func TestStartingAServerWithOneSleepIsNotPolling(t *testing.T) {
	require.False(t, hasPollingCalls([]fantasy.StepResult{
		toolStep("bash", `{"command":"npm run dev & sleep 3"}`),
		toolStep("bash", `{"command":"curl localhost:3000"}`),
	}))
}

func TestRestartAndBrowserVerificationIsNotPolling(t *testing.T) {
	steps := []fantasy.StepResult{
		toolStep("bash", `{"command":"Start-Sleep -Seconds 1; Start-Process PrimeAstra.exe; Start-Sleep -Seconds 3"}`),
		toolStep("mcp_browser_browser_navigate", `{"url":"http://127.0.0.1:43190"}`),
		toolStep("mcp_browser_browser_type", `{"ref":11,"text":"15 * 4","submit":true}`),
		toolStep("bash", `{"command":"Start-Sleep -Seconds 3"}`),
	}
	require.False(t, hasPollingCalls(steps))
}

func TestChangedReadResultsAreNotPolling(t *testing.T) {
	var steps []fantasy.StepResult
	for _, output := range []string{"pending", "starting", "running", "complete"} {
		steps = append(steps, makeToolStep("job_output", `{"shell_id":"build"}`, output))
	}
	require.False(t, hasPollingCalls(steps))
}

func TestEditAfterOldPollingResetsDetection(t *testing.T) {
	var steps []fantasy.StepResult
	for range 4 {
		steps = append(steps, toolStep("view", `{"file_path":"a"}`))
	}
	steps = append(steps, toolStep("edit", `{"file_path":"a"}`))
	require.False(t, hasPollingCalls(steps))
}

func TestEditsBetweenReadsAreNotPolling(t *testing.T) {
	var steps []fantasy.StepResult
	for range 4 {
		steps = append(steps, toolStep("view", `{"file_path":"a"}`), toolStep("edit", `{"file_path":"a"}`))
	}
	require.False(t, hasPollingCalls(steps))
}

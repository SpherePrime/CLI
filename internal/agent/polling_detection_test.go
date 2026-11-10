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

func TestEditsBetweenReadsAreNotPolling(t *testing.T) {
	var steps []fantasy.StepResult
	for range 4 {
		steps = append(steps, toolStep("view", `{"file_path":"a"}`), toolStep("edit", `{"file_path":"a"}`))
	}
	require.False(t, hasPollingCalls(steps))
}

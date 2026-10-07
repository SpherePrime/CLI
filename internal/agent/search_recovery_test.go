package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

func TestEmptySearchRecoveryPreservesOtherTools(t *testing.T) {
	var steps []fantasy.StepResult
	for i := range 6 {
		steps = append(steps, makeToolStep("grep", fmt.Sprintf(`{"pattern":"tools%d"}`, i%2), "No files found"))
	}
	newTool := func(name string) fantasy.AgentTool {
		return fantasy.NewAgentTool(name, name, func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})
	}
	allTools := []fantasy.AgentTool{newTool("grep"), newTool("glob"), newTool("view"), newTool("bash")}
	recovery := emptySearchRecovery{}
	prepared := fantasy.PrepareStepResult{Tools: allTools}
	recovery.prepare(steps, &prepared)
	if len(prepared.Messages) != 1 || len(prepared.Tools) != 3 {
		t.Fatalf("expected recovery guidance and three remaining tools, got %d messages and %d tools", len(prepared.Messages), len(prepared.Tools))
	}
	for index, name := range []string{"glob", "view", "bash"} {
		if prepared.Tools[index].Info().Name != name {
			t.Fatalf("remaining tool %d = %s, want %s", index, prepared.Tools[index].Info().Name, name)
		}
	}
	if allTools[0].Info().Name != "grep" {
		t.Fatal("recovery mutated the shared tool list")
	}
	prepared = fantasy.PrepareStepResult{Tools: allTools}
	recovery.prepare([]fantasy.StepResult{makeToolStep("view", `{"file":"main.go"}`, "source")}, &prepared)
	if len(prepared.Messages) != 1 || len(prepared.Tools) != 3 {
		t.Fatal("each recovered request must contain guidance and retain the restriction")
	}
	if hasRepeatedToolCalls(append(steps, makeToolStep("view", `{"file":"main.go"}`, "source")), 6, 2) {
		t.Fatal("old empty searches must not stop a recovered task")
	}
}

func TestEmptySearchRecoveryDoesNotInterruptProgress(t *testing.T) {
	tests := []struct {
		name string
		step func(int) fantasy.StepResult
	}{
		{"different patterns", func(i int) fantasy.StepResult {
			return makeToolStep("grep", fmt.Sprintf(`{"pattern":"missing%d"}`, i), "No files found")
		}},
		{"different scopes", func(i int) fantasy.StepResult {
			return makeToolStep("grep", fmt.Sprintf(`{"pattern":"tools","path":"directory%d"}`, i), "No files found")
		}},
		{"matching searches", func(i int) fantasy.StepResult {
			return makeToolStep("grep", `{"pattern":"tools"}`, "Found 1 matches")
		}},
		{"interleaved progress", func(i int) fantasy.StepResult {
			if i == 3 {
				return makeToolStep("edit", `{"file":"main.go"}`, "updated")
			}
			return makeToolStep("grep", `{"pattern":"tools"}`, "No files found")
		}},
		{"errors", func(i int) fantasy.StepResult {
			return makeToolStep("grep", `{"pattern":"tools"}`, "error searching files: invalid regex")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var steps []fantasy.StepResult
			for i := range 6 {
				steps = append(steps, test.step(i))
			}
			recovery := emptySearchRecovery{}
			prepared := fantasy.PrepareStepResult{}
			recovery.prepare(steps, &prepared)
			if recovery.active || len(prepared.Messages) != 0 {
				t.Fatal("normal search progress triggered recovery")
			}
		})
	}
}

func TestEmptySearchRecoveryNormalizesArguments(t *testing.T) {
	var steps []fantasy.StepResult
	for i := range 6 {
		input := `{"pattern":"tools", "path":"src"}`
		if i%2 == 0 {
			input = `{"path":"src","pattern":"tools"}`
		}
		steps = append(steps, makeToolStep("grep", input, "No files found"))
	}
	if !hasRepeatedEmptySearches(steps) {
		t.Fatal("JSON formatting must not hide repeated empty searches")
	}
}

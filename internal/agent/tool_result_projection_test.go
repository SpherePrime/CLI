package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestProjectToolMessagesPreservesLargeSkillViews(t *testing.T) {
	store := agingStore(t)

	var skillBuilder strings.Builder
	for i := range 3500 {
		fmt.Fprintf(&skillBuilder, "skill directive line %d\n", i)
	}
	largeSkill := skillBuilder.String()

	var normalBuilder strings.Builder
	for i := range 1500 {
		fmt.Fprintf(&normalBuilder, "normal file line %d\n", i)
	}
	largeNormal := normalBuilder.String()

	messages := []fantasy.Message{
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{fantasy.TextPart{Text: "read skills and files"}}},
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{
			fantasy.ToolCallPart{ToolCallID: "call-builtin-skill", ToolName: "view", Input: `{"file_path":"prime://skills/prime-designer/SKILL.md"}`},
			fantasy.ToolCallPart{ToolCallID: "call-local-skill", ToolName: "view", Input: `{"file_path":"/Users/dev/.config/prime/skills/custom/SKILL.md"}`},
			fantasy.ToolCallPart{ToolCallID: "call-ref-skill", ToolName: "view", Input: `{"file_path":"skills/custom/references/quality-gate.md"}`},
			fantasy.ToolCallPart{ToolCallID: "call-normal-view", ToolName: "view", Input: `{"file_path":"internal/main.go"}`},
			fantasy.ToolCallPart{ToolCallID: "call-bash", ToolName: "bash", Input: `{"command":"cat big.log"}`},
		}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{
			fantasy.ToolResultPart{ToolCallID: "call-builtin-skill", Outcome: fantasy.ToolOutcomeSuccess, Output: fantasy.ToolResultOutputContentText{Text: largeSkill}},
			fantasy.ToolResultPart{ToolCallID: "call-local-skill", Outcome: fantasy.ToolOutcomeSuccess, Output: fantasy.ToolResultOutputContentText{Text: largeSkill}},
			fantasy.ToolResultPart{ToolCallID: "call-ref-skill", Outcome: fantasy.ToolOutcomeSuccess, Output: fantasy.ToolResultOutputContentText{Text: largeSkill}},
			fantasy.ToolResultPart{ToolCallID: "call-normal-view", Outcome: fantasy.ToolOutcomeSuccess, Output: fantasy.ToolResultOutputContentText{Text: largeNormal}},
			fantasy.ToolResultPart{ToolCallID: "call-bash", Outcome: fantasy.ToolOutcomeSuccess, Output: fantasy.ToolResultOutputContentText{Text: largeNormal}},
		}},
	}

	projected := projectToolMessages(context.Background(), "scope", store, messages)
	parts := projected[2].Content

	require.Equal(t, largeSkill, parts[0].(fantasy.ToolResultPart).Output.(fantasy.ToolResultOutputContentText).Text)
	require.Equal(t, largeSkill, parts[1].(fantasy.ToolResultPart).Output.(fantasy.ToolResultOutputContentText).Text)
	require.Equal(t, largeSkill, parts[2].(fantasy.ToolResultPart).Output.(fantasy.ToolResultOutputContentText).Text)

	normalViewText := parts[3].(fantasy.ToolResultPart).Output.(fantasy.ToolResultOutputContentText).Text
	require.Contains(t, normalViewText, "result shortened")
	require.Contains(t, normalViewText, readToolResultName)

	bashText := parts[4].(fantasy.ToolResultPart).Output.(fantasy.ToolResultOutputContentText).Text
	require.Contains(t, bashText, "result shortened")
	require.Contains(t, bashText, readToolResultName)
}

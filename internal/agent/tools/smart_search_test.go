package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dwertyfa288/CLI/internal/skills"
	"github.com/dwertyfa288/CLI/vendordeps/fantasy"
	"github.com/dwertyfa288/CLI/vendordeps/stretchr/testify/require"
)

func runSearchTool(t *testing.T, tool fantasy.AgentTool, input string) string {
	t.Helper()
	result, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  tool.Info().Name,
		Input: input,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Content)
	return result.Content
}

func TestSearchSkillsToolReturnsUsageInstructions(t *testing.T) {
	t.Parallel()

	all := []*skills.Skill{
		{
			Name:          "git-workflow",
			Description:   "Use when committing and rebasing git branches",
			Instructions:  "Always sign commits.",
			SkillFilePath: "/skills/git-workflow/SKILL.md",
		},
		{
			Name:          "jq",
			Description:   "Transform JSON data",
			SkillFilePath: "/skills/jq/SKILL.md",
		},
	}

	tool := NewSearchSkillsTool(all, nil)
	require.Equal(t, "search_skills", tool.Info().Name)
	require.Contains(t, tool.Info().Required, "query")

	out := runSearchTool(t, tool, `{"query":"rebase branch"}`)
	require.Contains(t, out, "git-workflow")
	require.Contains(t, out, "/skills/git-workflow/SKILL.md")
	require.Contains(t, out, "How to use:")
	require.NotContains(t, out, "jq")
}

func TestSearchSkillsToolHonorsDisabledAndInternalSkills(t *testing.T) {
	t.Parallel()

	all := []*skills.Skill{
		{Name: "hidden", Description: "commit workflow", SkillFilePath: "/a/SKILL.md"},
		{Name: "blocked", Description: "commit workflow", SkillFilePath: "/b/SKILL.md"},
		{Name: "model-off", Description: "commit workflow", DisableModelInvocation: true, SkillFilePath: "/c/SKILL.md"},
	}

	tool := NewSearchSkillsTool(all, []string{"blocked"})
	out := runSearchTool(t, tool, `{"query":"commit workflow"}`)
	require.Contains(t, out, "hidden")
	require.NotContains(t, out, "blocked")
	require.NotContains(t, out, "model-off")
}

func TestSearchMCPToolReportsCallableNamesAndRequiredParams(t *testing.T) {
	t.Parallel()

	entries := []SmartSearchEntry{
		{
			Kind:        "mcp",
			Name:        "mcp_github-create_issue",
			Description: "Create an issue in a repository",
			Instruction: "Call mcp_github-create_issue with JSON parameters. Required parameters: title, repo.",
		},
		{
			Kind:        "mcp",
			Name:        "mcp_context7-fetch_docs",
			Description: "Fetch library documentation",
			Instruction: "Call mcp_context7-fetch_docs with JSON parameters.",
		},
	}

	out := runSearchTool(t, NewSearchMCPTool(entries), `{"query":"create issue"}`)
	require.Contains(t, out, "mcp_github-create_issue")
	require.Contains(t, out, "Required parameters: title, repo")
	require.NotContains(t, out, "mcp_context7-fetch_docs")
}

func TestSearchToolsToolMatchesDescription(t *testing.T) {
	t.Parallel()

	entries := []SmartSearchEntry{
		{Kind: "tool", Name: "grep", Description: "Search file contents by pattern", Instruction: "Call grep using its JSON parameters."},
		{Kind: "tool", Name: "view", Description: "Read a file with line numbers", Instruction: "Call view using its JSON parameters."},
	}

	out := runSearchTool(t, NewSearchToolsTool(entries), `{"query":"contents by pattern"}`)
	require.Contains(t, out, "tool grep")
	require.NotContains(t, out, "view")
}

func TestSearchToolWithoutQueryAsksForKeyword(t *testing.T) {
	t.Parallel()

	out := runSearchTool(t, NewSearchToolsTool(nil), `{"query":"  "}`)
	require.Contains(t, out, "No query provided")
}

func TestSearchToolRespectsLimit(t *testing.T) {
	t.Parallel()

	entries := make([]SmartSearchEntry, 0, 5)
	for i := range 5 {
		entries = append(entries, SmartSearchEntry{
			Kind: "tool",
			Name: "tool" + string(rune('a'+i)),
		})
	}

	out := runSearchTool(t, NewSearchToolsTool(entries), `{"query":"tool","limit":2}`)
	require.Contains(t, out, "1. tool toola")
	require.Contains(t, out, "2. tool toolb")
	require.NotContains(t, out, "3. tool")
}

func TestSmartSearchTermsDropsStopWords(t *testing.T) {
	t.Parallel()

	terms := smartSearchTerms("how do I handle merge conflicts in files")
	require.NotContains(t, terms, "the")
	require.NotContains(t, terms, "in")
	require.Contains(t, terms, "conflicts")
	require.Equal(t, "conflicts", terms[0])
}

func TestSmartSearchEntriesFromToolsSkipsUnnamed(t *testing.T) {
	t.Parallel()

	entries := SmartSearchEntriesFromTools([]fantasy.AgentTool{
		NewSearchToolsTool(nil),
		&unnamedTool{},
	})
	require.Len(t, entries, 1)
	require.Equal(t, "search_tools", entries[0].Name)
	require.Equal(t, "tool", entries[0].Kind)

	payload, err := json.Marshal(entries[0])
	require.NoError(t, err)
	require.Contains(t, string(payload), `"kind":"tool"`)
}

func TestSmartSearchEntriesFromSkillsSummarizesInstructions(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("word ", 200)
	entries := SmartSearchEntriesFromSkills([]*skills.Skill{
		{Name: "big", Description: "d", Instructions: long, SkillFilePath: "/big/SKILL.md"},
	}, nil)

	require.Len(t, entries, 1)
	require.Less(t, len(entries[0].Instruction), len(long))
	require.True(t, strings.HasSuffix(entries[0].Instruction, "…"))
}

// unnamedTool implements fantasy.AgentTool with an empty name to make sure
// catalog building ignores unusable entries.
type unnamedTool struct{}

func (unnamedTool) Info() fantasy.ToolInfo { return fantasy.ToolInfo{} }

func (unnamedTool) Run(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.ToolResponse{}, nil
}

func (unnamedTool) ProviderOptions() fantasy.ProviderOptions { return nil }

func (unnamedTool) SetProviderOptions(fantasy.ProviderOptions) {}

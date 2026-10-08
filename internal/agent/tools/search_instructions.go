package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SpherePrime/CLI/internal/instructions"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const SearchInstructionsToolName = "search_instructions"

type SearchInstructionsParams struct {
	Query string `json:"query,omitempty" description:"Task keywords in English or Russian; omit to browse the instruction catalog"`
	Limit int    `json:"limit,omitempty" description:"Maximum metadata matches, default 6 and maximum 12"`
}

type instructionSearchMatch struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Read        string `json:"read"`
}

func NewSearchInstructionsTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(SearchInstructionsToolName,
		"Find task-specific instructions in Prime's bundled library. Returns metadata only, never instruction bodies. Search before work begins and when the task changes, then read the relevant IDs with read_instruction. No network access is required.",
		func(ctx context.Context, params SearchInstructionsParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if err := ctx.Err(); err != nil {
				return fantasy.ToolResponse{}, err
			}
			matches := make([]instructionSearchMatch, 0)
			for _, entry := range instructions.Search(params.Query, params.Limit) {
				input, _ := json.Marshal(map[string]string{"id": entry.ID})
				matches = append(matches, instructionSearchMatch{
					ID: entry.ID, Title: entry.Title, Description: entry.Description, Category: entry.Category,
					Read: fmt.Sprintf("read_instruction(%s)", input),
				})
			}
			result := struct {
				Categories []string                 `json:"categories"`
				Matches    []instructionSearchMatch `json:"matches"`
				Guidance   string                   `json:"guidance"`
			}{instructions.Categories(), matches, "Read only the relevant matches before acting. If no match exists, try one broader synonym, then proceed with available tools; do not repeat empty searches."}
			content, err := json.Marshal(result)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			return fantasy.NewTextResponse(string(content)), nil
		})
}

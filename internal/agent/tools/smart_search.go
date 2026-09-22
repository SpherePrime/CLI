package tools

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/internal/skills"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const SmartSearchDefaultLimit = 8

// SmartSearchEntry is a catalog entry for tool-search mode.
type SmartSearchEntry struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
}

// SmartSearchHit is a single ranked result from a keyword search.
type SmartSearchHit struct {
	Entry   SmartSearchEntry `json:"entry"`
	Reasons []string         `json:"reasons,omitempty"`
}

// SearchSkillsParams is the input for the search_skills agent tool.
type SearchSkillsParams struct {
	Query string `json:"query" description:"Keyword or short phrase describing the skill to find"`
	Limit int    `json:"limit,omitempty" description:"Optional maximum number of results to return. Defaults to 8"`
}

// SearchMCPParams is the input for the search_mcp agent tool.
type SearchMCPParams struct {
	Query string `json:"query" description:"Keyword or short phrase describing the MCP tool or server to find"`
	Limit int    `json:"limit,omitempty" description:"Optional maximum number of results to return. Defaults to 8"`
}

// SearchToolsParams is the input for the search_tools agent tool.
type SearchToolsParams struct {
	Query string `json:"query" description:"Keyword or short phrase describing a built-in agent tool to find"`
	Limit int    `json:"limit,omitempty" description:"Optional maximum number of results to return. Defaults to 8"`
}

var smartSearchStopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true,
	"of": true, "for": true, "with": true, "in": true, "on": true,
	"to": true, "how": true, "what": true, "which": true, "is": true,
	"are": true, "was": true, "were": true, "find": true, "search": true,
	"using": true, "use": true, "by": true, "from": true, "that": true,
	"this": true, "it": true, "you": true, "i": true,
}

var smartSearchTokenPattern = regexp.MustCompile(`[a-z0-9_\-]+`)

// NewSearchSkillsTool returns the search_skills agent tool.
func NewSearchSkillsTool(all []*skills.Skill, disabledSkills []string) fantasy.AgentTool {
	entries := SmartSearchEntriesFromSkills(all, disabledSkills)
	return fantasy.NewAgentTool("search_skills",
		"Search available skills by keyword and return the best matches with instructions on how to load them. Use this when a matching skill may exist but is not already visible in the request.",
		func(ctx context.Context, params SearchSkillsParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse(smartSearchRun(entries, params.Query, params.Limit)), nil
		})
}

// NewSearchMCPTool returns the search_mcp agent tool.
func NewSearchMCPTool(entries []SmartSearchEntry) fantasy.AgentTool {
	return fantasy.NewAgentTool("search_mcp",
		"Search MCP tools by keyword and return matching tools with their mcp_<server>_<tool> names, descriptions, and short usage notes. Use this when an external MCP capability may satisfy the request.",
		func(ctx context.Context, params SearchMCPParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse(smartSearchRun(entries, params.Query, params.Limit)), nil
		})
}

// NewSearchToolsTool returns the search_tools agent tool.
func NewSearchToolsTool(entries []SmartSearchEntry) fantasy.AgentTool {
	return fantasy.NewAgentTool("search_tools",
		"Search built-in agent tools by keyword and return matching tools with descriptions and usage notes. Use this when a built-in tool may satisfy the request.",
		func(ctx context.Context, params SearchToolsParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse(smartSearchRun(entries, params.Query, params.Limit)), nil
		})
}

// SmartSearchEntriesFromSkills converts available skills into searchable catalog entries.
func SmartSearchEntriesFromSkills(all []*skills.Skill, disabled []string) []SmartSearchEntry {
	available := skills.Filter(all, disabled)
	entries := make([]SmartSearchEntry, 0, len(available))
	for _, s := range available {
		if s.DisableModelInvocation {
			continue
		}
		instruction := fmt.Sprintf(
			"Read the skill with the view tool at %s, then follow its instructions. Skill name: %s.",
			s.SkillFilePath, s.Name,
		)
		if s.Instructions != "" {
			instruction = fmt.Sprintf(
				"Read the skill with the view tool at %s before acting. Skill name: %s. Summary: %s",
				s.SkillFilePath, s.Name, summarizeSmartSearchText(s.Instructions, 280),
			)
		}
		entries = append(entries, SmartSearchEntry{
			Kind:        "skill",
			Name:        s.Name,
			Description: s.Description,
			Instruction: instruction,
		})
	}
	return entries
}

// SmartSearchEntriesFromTools converts a slice of fantasy.AgentTool instances
// into searchable catalog entries for tool-search mode.
func SmartSearchEntriesFromTools(all []fantasy.AgentTool) []SmartSearchEntry {
	entries := make([]SmartSearchEntry, 0, len(all))
	for _, tool := range all {
		info := tool.Info()
		if info.Name == "" {
			continue
		}
		entries = append(entries, SmartSearchEntry{
			Kind:        "tool",
			Name:        info.Name,
			Description: info.Description,
			Instruction: fmt.Sprintf("Call %s using its JSON parameters.", info.Name),
		})
	}
	return entries
}

func smartSearchRun(entries []SmartSearchEntry, query string, limit int) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "No query provided. Provide a keyword or short phrase to search for."
	}
	if limit <= 0 {
		limit = SmartSearchDefaultLimit
	}

	terms := smartSearchTerms(query)
	matches := make([]SmartSearchHit, 0, limit)
	for _, entry := range entries {
		if !smartSearchEntryVisible(entry, terms) {
			continue
		}
		matches = append(matches, SmartSearchHit{
			Entry:   entry,
			Reasons: smartSearchEntryReasons(entry, terms),
		})
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Query: %q\n", query)
	if len(matches) == 0 {
		sb.WriteString("No matches found.\n")
		sb.WriteString("Try a broader keyword, a different noun, or search another category (skills, MCP tools, built-in tools).\n")
		return sb.String()
	}
	for i, hit := range matches {
		fmt.Fprintf(&sb, "\n%d. %s %s\n", i+1, hit.Entry.Kind, hit.Entry.Name)
		if hit.Entry.Description != "" {
			fmt.Fprintf(&sb, "   Description: %s\n", summarizeSmartSearchText(hit.Entry.Description, 320))
		}
		if len(hit.Reasons) > 0 {
			fmt.Fprintf(&sb, "   Matched: %s\n", strings.Join(hit.Reasons, ", "))
		}
		if hit.Entry.Instruction != "" {
			fmt.Fprintf(&sb, "   How to use: %s\n", summarizeSmartSearchText(hit.Entry.Instruction, 360))
		}
	}
	return sb.String()
}

func smartSearchEntryVisible(entry SmartSearchEntry, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(entry.Name + " " + entry.Description + " " + entry.Instruction)
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			return true
		}
	}
	return false
}

func smartSearchEntryReasons(entry SmartSearchEntry, terms []string) []string {
	var reasons []string
	name := strings.ToLower(entry.Name)
	for _, term := range terms {
		if strings.Contains(name, term) {
			reasons = append(reasons, "name contains "+term)
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "matched description or instructions")
	}
	return reasons
}

func smartSearchTerms(query string) []string {
	query = strings.ToLower(query)
	var terms []string
	for _, token := range smartSearchTokenPattern.FindAllString(query, -1) {
		if token == "-" || len(token) < 2 {
			continue
		}
		if smartSearchStopWords[token] {
			continue
		}
		terms = append(terms, token)
	}
	slices.SortFunc(terms, func(a, b string) int {
		if len(a) == len(b) {
			return strings.Compare(a, b)
		}
		return len(b) - len(a)
	})
	return terms
}

func summarizeSmartSearchText(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}

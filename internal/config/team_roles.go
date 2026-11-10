package config

const (
	AgentExplore string = "explore"
	AgentReview  string = "review"
)

var reviewForbiddenTools = []string{
	"agent",
	"agent_jobs",
	"sudo",
	"edit",
	"multiedit",
	"write",
	"lsp_rename",
	"lsp_replace_symbol",
	"generate_image",
	"goal_complete",
}

func resolveExploreTools(tools []string) []string {
	exploreTools := []string{
		"agent_message",
		"search_instructions",
		"read_instruction",
		"bash",
		"job_output",
		"job_kill",
		"glob",
		"grep",
		"ls",
		"view",
		"lsp_call_hierarchy",
		"lsp_definition",
		"lsp_references",
		"lsp_symbols",
		"lsp_diagnostics",
		"sourcegraph",
		"fetch",
	}
	return filterSlice(tools, exploreTools, true)
}

func resolveReviewTools(tools []string) []string {
	return filterSlice(tools, reviewForbiddenTools, false)
}

func teamRoleAgents(allowedTools []string, contextPaths []string) map[string]Agent {
	return map[string]Agent{
		AgentExplore: {
			ID:           AgentExplore,
			Name:         "Explore",
			Description:  "Collects project context before work is split: structure, conventions, build and test commands, relevant code. Does not change files.",
			Model:        SelectedModelTypeLarge,
			ContextPaths: contextPaths,
			AllowedTools: resolveExploreTools(allowedTools),
			AllowedMCP:   map[string][]string{},
		},
		AgentReview: {
			ID:           AgentReview,
			Name:         "Review",
			Description:  "Strictly checks finished work: builds, tests, runs it, takes screenshots and returns a verdict with defects. Does not change files.",
			Model:        SelectedModelTypeLarge,
			ContextPaths: contextPaths,
			AllowedTools: resolveReviewTools(allowedTools),
		},
	}
}

func inheritExploreModel(agents map[string]Agent) {
	explore, ok := agents[AgentExplore]
	if !ok || explore.ModelOverride != nil {
		return
	}
	task, ok := agents[AgentTask]
	if !ok || task.ModelOverride == nil {
		return
	}
	pinned := *task.ModelOverride
	explore.ModelOverride = &pinned
	agents[AgentExplore] = explore
}

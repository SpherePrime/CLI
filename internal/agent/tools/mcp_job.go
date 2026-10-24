package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	MCPJobToolName    = "mcp_job"
	mcpHealthToolName = "browser_health"
	mcpProbeWait      = 5 * time.Second
	mcpProgressFresh  = 60 * time.Second
	mcpPageHungAfter  = 10 * time.Second
)

//go:embed mcp_job.md
var mcpJobDescription string

type MCPJobParams struct {
	Op    string `json:"op,omitempty" description:"status (default), list, result or kill"`
	JobID string `json:"job_id,omitempty" description:"Background MCP job ID, for example mcp-3"`
}

type pageHealth struct {
	Responsive          bool    `json:"responsive"`
	UnresponsiveSeconds float64 `json:"unresponsiveSeconds"`
	Evaluating          bool    `json:"evaluating"`
	EvaluatingSeconds   float64 `json:"evaluatingSeconds"`
}

func NewMCPJobTool(cfg *config.ConfigStore) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MCPJobToolName,
		mcpJobDescription,
		func(ctx context.Context, params MCPJobParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if params.Op == "list" {
				return fantasy.NewTextResponse(listMCPJobs(sessionID)), nil
			}
			if params.JobID == "" {
				return fantasy.NewTextErrorResponse("missing job_id"), nil
			}
			job, ok := mcpJobs.Get(params.JobID)
			if !ok || job.sessionID != sessionID {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("background MCP job not found: %s", params.JobID)), nil
			}
			switch params.Op {
			case "", "status":
				return fantasy.NewTextResponse(mcpJobStatus(ctx, cfg, job)), nil
			case "result":
				status, result, err, _ := job.snapshot()
				if status == "running" {
					return fantasy.NewTextResponse(fmt.Sprintf("Job %s is still running. Use op=status to check whether it is progressing.", job.id)), nil
				}
				job.markSeen()
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return mcpResponse(ctx, result), nil
			case "kill":
				job.kill()
				return fantasy.NewTextResponse(fmt.Sprintf("Cancellation sent to %s (%s).", job.id, job.label)), nil
			default:
				return fantasy.NewTextErrorResponse("op must be status, list, result or kill"), nil
			}
		},
	)
}

func listMCPJobs(sessionID string) string {
	jobs := sessionMCPJobs(sessionID)
	if len(jobs) == 0 {
		return "No background MCP jobs in this session."
	}
	var b strings.Builder
	for _, job := range jobs {
		status, _, _, finishedAt := job.snapshot()
		elapsed := time.Since(job.startedAt)
		if status != "running" {
			elapsed = finishedAt.Sub(job.startedAt)
		}
		fmt.Fprintf(&b, "%s  %s  %s  %s\n", job.id, job.label, status, elapsed.Round(time.Second))
	}
	return strings.TrimSpace(b.String())
}

func mcpJobStatus(ctx context.Context, cfg *config.ConfigStore, job *mcpJob) string {
	status, _, err, finishedAt := job.snapshot()
	var b strings.Builder
	if status != "running" {
		fmt.Fprintf(&b, "Job %s (%s) finished with status %s after %s.", job.id, job.label, status, finishedAt.Sub(job.startedAt).Round(time.Second))
		if err != nil {
			fmt.Fprintf(&b, "\nError: %s", err)
		}
		b.WriteString("\nUse op=result to read the result.")
		return b.String()
	}
	fmt.Fprintf(&b, "Job %s (%s) is running for %s.\n", job.id, job.label, time.Since(job.startedAt).Round(time.Second))

	started := time.Now()
	pingErr := mcp.Ping(ctx, job.mcpName, mcpProbeWait)
	if pingErr != nil {
		fmt.Fprintf(&b, "Server: does not respond to ping (%s).\n", pingErr)
	} else {
		fmt.Fprintf(&b, "Server: responds to ping in %s.\n", time.Since(started).Round(time.Millisecond))
	}

	progress, hasProgress := mcp.LastProgress(job.id)
	freshProgress := hasProgress && time.Since(progress.At) < mcpProgressFresh
	if hasProgress {
		fmt.Fprintf(&b, "Progress: last report %s ago", time.Since(progress.At).Round(time.Second))
		if progress.Total > 0 {
			fmt.Fprintf(&b, " (%g/%g)", progress.Value, progress.Total)
		}
		if progress.Message != "" {
			fmt.Fprintf(&b, ": %s", progress.Message)
		}
		b.WriteString(".\n")
	} else {
		b.WriteString("Progress: the server sends no progress reports for this call.\n")
	}

	page, hasPage := probePage(ctx, cfg, job)
	if hasPage {
		if page.Responsive {
			b.WriteString("Page: responds to the browser.")
		} else {
			fmt.Fprintf(&b, "Page: main thread does not respond for %.0fs.", page.UnresponsiveSeconds)
		}
		if page.Evaluating {
			fmt.Fprintf(&b, " Script running for %.0fs.", page.EvaluatingSeconds)
		}
		b.WriteString("\n")
	}

	switch {
	case pingErr != nil:
		b.WriteString("Verdict: likely hung — the server is not responding. Kill the job and retry or report the failure.")
	case hasPage && !page.Responsive && page.UnresponsiveSeconds >= mcpPageHungAfter.Seconds():
		b.WriteString("Verdict: likely hung — the page script blocks the page. Kill the job, then reload the tab or use a lighter script.")
	case freshProgress:
		b.WriteString("Verdict: progressing — wait for the completion notice.")
	case hasPage && page.Responsive:
		b.WriteString("Verdict: running — the server and page respond; the script is still waiting for its result. Kill it if the result is no longer needed.")
	default:
		b.WriteString("Verdict: running — the server responds but reports no progress. Wait for the completion notice or kill the job if it is no longer needed.")
	}
	return b.String()
}

func probePage(ctx context.Context, cfg *config.ConfigStore, job *mcpJob) (pageHealth, bool) {
	if !strings.HasPrefix(job.toolName, "browser_") || !mcp.HasTool(job.mcpName, mcpHealthToolName) {
		return pageHealth{}, false
	}
	var input map[string]any
	_ = json.Unmarshal([]byte(job.input), &input)
	args := map[string]any{}
	if tab, ok := input["tabId"].(string); ok && tab != "" {
		args["tabId"] = tab
	}
	encoded, _ := json.Marshal(args)
	probeCtx, cancel := context.WithTimeout(ctx, 2*mcpProbeWait)
	defer cancel()
	result, err := mcp.RunTool(probeCtx, cfg, job.mcpName, mcpHealthToolName, string(encoded), job.sessionID)
	if err != nil {
		return pageHealth{}, false
	}
	var health pageHealth
	if json.Unmarshal([]byte(result.Content), &health) != nil {
		return pageHealth{}, false
	}
	return health, true
}

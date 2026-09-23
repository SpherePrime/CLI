package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/internal/shell"
)

const (
	JobOutputToolName = "job_output"

	// jobOutputWaitCap bounds how long job_output(wait=true) blocks, so a
	// job that never exits cannot stall the agent run.
	jobOutputWaitCap = 30 * time.Second
)

//go:embed job_output.md
var jobOutputDescription string

type JobOutputParams struct {
	ShellID string `json:"shell_id" description:"The ID of the background shell to retrieve output from"`
	Wait    bool   `json:"wait" description:"If true, wait for the shell to finish, up to a bounded timeout. A still-running job returns its current output with status running"`
}

type JobOutputResponseMetadata struct {
	ShellID          string `json:"shell_id"`
	Command          string `json:"command"`
	Description      string `json:"description"`
	Done             bool   `json:"done"`
	WorkingDirectory string `json:"working_directory"`
}

func NewJobOutputTool(spillDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobOutputToolName,
		jobOutputDescription,
		func(ctx context.Context, params JobOutputParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ShellID == "" {
				return fantasy.NewTextErrorResponse("missing shell_id"), nil
			}

			bgManager := shell.GetBackgroundShellManager()
			bgShell, ok := bgManager.Get(params.ShellID)
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("background shell not found: %s", params.ShellID)), nil
			}

			timedOut := false
			if params.Wait {
				// Bound the wait so a job that never exits (server, watch
				// loop) cannot stall the agent. Agent-context cancellation
				// still wins immediately, so ESC cancels the wait at once.
				timer := time.NewTimer(jobOutputWaitCap)
				select {
				case <-bgShell.Done():
				case <-ctx.Done():
				case <-timer.C:
					timedOut = true
				}
				timer.Stop()
			}

			stdout, stderr, done, err := bgShell.GetOutput()

			var outputParts []string
			if stdout != "" {
				outputParts = append(outputParts, stdout)
			}
			if stderr != "" {
				outputParts = append(outputParts, stderr)
			}

			status := "running"
			if done {
				status = "completed"
				if err != nil {
					exitCode := shell.ExitCode(err)
					if exitCode != 0 {
						outputParts = append(outputParts, fmt.Sprintf("Exit code %d", exitCode))
					}
				}
			} else if timedOut {
				status = fmt.Sprintf("running (still running after %s; use job_kill to terminate)", jobOutputWaitCap)
			}

			output := strings.Join(outputParts, "\n")
			output = TruncateOutput(output, spillDir)

			metadata := JobOutputResponseMetadata{
				ShellID:          params.ShellID,
				Command:          bgShell.Command,
				Description:      bgShell.Description,
				Done:             done,
				WorkingDirectory: bgShell.WorkingDir,
			}

			if output == "" {
				output = BashNoOutput
			}

			result := fmt.Sprintf("Status: %s\n\n%s", status, output)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		},
	)
}

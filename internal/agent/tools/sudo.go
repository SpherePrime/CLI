package tools

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/shell"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

type SudoParams struct {
	Description string `json:"description" description:"A brief description of what the command does, try to keep it under 30 characters or so"`
	Command     string `json:"command" description:"The command to execute with root privileges, without the sudo prefix"`
	WorkingDir  string `json:"working_dir,omitempty" description:"The working directory to execute the command in (defaults to current directory)"`
	Timeout     int    `json:"timeout,omitempty" description:"Optional timeout in seconds (default 120, max 600)"`
}

type SudoPermissionsParams struct {
	Description string `json:"description"`
	Command     string `json:"command"`
	WorkingDir  string `json:"working_dir,omitempty"`
	Timeout     int    `json:"timeout,omitempty"`
}

const (
	SudoToolName = "sudo"

	DefaultSudoTimeout = 120
	MaxSudoTimeout     = 600

	sudoPasswordEnv = "PRIME_DESKTOP_SUDO_PASSWORD"
	// SudoSettingsLink is the markdown link the tool asks the agent to repeat
	// when the root password is not configured. The desktop chat opens the
	// settings dialog on the sudo tab for prime:// links.
	SudoSettingsLink = "[Sudo settings](prime://settings/sudo)"
)

//go:embed sudo.md.tpl
var sudoDescriptionTmpl []byte

var sudoDescriptionTpl = template.Must(
	template.New("sudoDescription").
		Parse(string(sudoDescriptionTmpl)),
)

type sudoDescriptionData struct {
	DefaultTimeout int
	MaxTimeout     int
}

func sudoDescription() string {
	return renderTemplate(sudoDescriptionTpl, sudoDescriptionData{
		DefaultTimeout: DefaultSudoTimeout,
		MaxTimeout:     MaxSudoTimeout,
	})
}

func sudoPassword() string {
	return os.Getenv(sudoPasswordEnv)
}

var sudoToolAvailable = runtime.GOOS == "linux"

func blockedRootHint(err error) string {
	if err == nil || !strings.Contains(err.Error(), "not allowed for security reasons") {
		return ""
	}
	if !sudoToolAvailable {
		return ""
	}
	return "\n\nThis command was blocked. If it needs root privileges, run it through the sudo tool (pass the command WITHOUT the sudo prefix) instead of asking the user."
}

// sudoNotConfiguredResponse explains what is missing and tells the agent to
// guide the user to the settings instead of retrying.
func sudoNotConfiguredResponse() fantasy.ToolResponse {
	message := "The sudo tool is not configured: the root password for this computer is not set. " +
		"Tell the user that running commands with root privileges needs setup and include this exact markdown link in your reply: " +
		SudoSettingsLink + " " +
		"The link opens the settings dialog on the sudo tab where the root password is set. " +
		"Do not call sudo again until the user has saved the settings."
	return fantasy.NewTextErrorResponse(message)
}

// writeAskpassScript writes a temporary askpass script that prints the root
// password. The password never appears on a command line: sudo reads it from
// the script through SUDO_ASKPASS, and the file is removed by the caller.
func writeAskpassScript(password string) (string, error) {
	file, err := os.CreateTemp("", "prime-sudo-askpass-")
	if err != nil {
		return "", fmt.Errorf("failed to create askpass script: %w", err)
	}
	script := "#!/bin/sh\necho '" + strings.ReplaceAll(password, "'", `'\''`) + "'\n"
	if _, err := file.WriteString(script); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", fmt.Errorf("failed to write askpass script: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("failed to write askpass script: %w", err)
	}
	if err := os.Chmod(file.Name(), 0o700); err != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("failed to secure askpass script: %w", err)
	}
	return file.Name(), nil
}

// sudoCommand wraps the user command so it runs as root through sudo with
// the askpass program. -p '' silences the password prompt in the output.
func sudoCommand(command, askpassPath string) string {
	quoted := "'" + strings.ReplaceAll(command, "'", `'\''`) + "'"
	return "sudo -A -p '' -- bash -c " + quoted
}

func NewSudoTool(permissions permission.Service, workingDir, spillDir string) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		SudoToolName,
		sudoDescription(),
		func(ctx context.Context, params SudoParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Command) == "" {
				return fantasy.NewTextErrorResponse("missing command"), nil
			}

			password := sudoPassword()
			if password == "" {
				return sudoNotConfiguredResponse(), nil
			}

			execWorkingDir := params.WorkingDir
			if execWorkingDir == "" {
				execWorkingDir = workingDir
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for executing sudo commands")
			}

			allowed, err := permissions.Request(
				ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        execWorkingDir,
					ToolCallID:  call.ID,
					ToolName:    SudoToolName,
					Action:      "execute",
					Description: fmt.Sprintf("Execute command as root: %s", params.Command),
					Params:      SudoPermissionsParams(params),
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !allowed {
				return NewPermissionDeniedResponse(), nil
			}

			timeout := params.Timeout
			if timeout <= 0 {
				timeout = DefaultSudoTimeout
			}
			if timeout > MaxSudoTimeout {
				timeout = MaxSudoTimeout
			}
			runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()

			askpassPath, err := writeAskpassScript(password)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			defer os.Remove(askpassPath)

			env := append(os.Environ(), "SUDO_ASKPASS="+askpassPath)
			executor := shell.NewShell(&shell.Options{
				WorkingDir: execWorkingDir,
				Env:        env,
			})

			type result struct {
				stdout string
				stderr string
				err    error
			}
			done := make(chan result, 1)
			go func() {
				stdout, stderr, execErr := executor.Exec(runCtx, sudoCommand(params.Command, askpassPath))
				done <- result{stdout, stderr, execErr}
			}()

			select {
			case outcome := <-done:
				stdout := formatOutput(outcome.stdout, outcome.stderr, outcome.err, spillDir)
				stdout += fmt.Sprintf("\n\n<cwd>%s</cwd>", normalizeWorkingDir(execWorkingDir))
				return fantasy.NewTextResponse(stdout), nil
			case <-runCtx.Done():
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Command did not finish within %d seconds", timeout)), nil
			}
		},
	)
}

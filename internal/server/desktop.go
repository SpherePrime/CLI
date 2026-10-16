package server

import (
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/question"
	"github.com/SpherePrime/CLI/internal/shell"
	"net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func (c *controllerV1) handleGetCapabilities(w http.ResponseWriter, r *http.Request) {
	jsonEncode(w, proto.DesktopCapabilities{ProtocolVersion: 1, RunRegistry: true, RunRegistryPersistence: "sqlite", PendingRequests: true, ModelCapabilities: true, Plugins: true, VoiceUpload: true})
}

func (c *controllerV1) handleGetPending(w http.ResponseWriter, r *http.Request) {
	ws, err := c.backend.GetWorkspace(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	permissions := []permission.PermissionRequest{}
	var pendingQuestion *question.Request
	if service, ok := ws.Permissions.(interface {
		Pending() []permission.PermissionRequest
	}); ok {
		permissions = service.Pending()
	}
	if service, ok := ws.Questions.(interface{ Pending() *question.Request }); ok {
		pendingQuestion = service.Pending()
	}
	jsonEncode(w, struct {
		Permissions []permission.PermissionRequest `json:"permissions"`
		Question    *question.Request              `json:"question"`
	}{permissions, pendingQuestion})
}

func (c *controllerV1) handleGetRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := c.backend.ListRuns(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, runs)
}

func (c *controllerV1) handleGetRun(w http.ResponseWriter, r *http.Request) {
	run, err := c.backend.GetRun(r.PathValue("id"), r.PathValue("run_id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, run)
}

func (c *controllerV1) handleGetModelCapabilities(w http.ResponseWriter, r *http.Request) {
	cfg, err := c.backend.GetWorkspaceConfig(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	models := []proto.ModelCapabilities{}
	for providerID, provider := range cfg.Providers.Seq2() {
		if provider.Disable {
			continue
		}
		for _, model := range provider.Models {
			modes := []string{config.ReasoningModeOff}
			if model.CanReason {
				modes = append(modes, config.AllReasoningModes()...)
			}
			models = append(models, proto.ModelCapabilities{ProviderID: providerID, ModelID: model.ID, Name: model.Name, CanReason: model.CanReason, ReasoningModes: modes, ReasoningLevels: model.ReasoningLevels, SupportsAttachments: model.SupportsImages, ContextWindow: model.ContextWindow, DefaultMaxTokens: model.DefaultMaxTokens})
		}
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].ProviderID+"/"+models[i].ModelID < models[j].ProviderID+"/"+models[j].ModelID
	})
	jsonEncode(w, models)
}

func (c *controllerV1) handleGetJobs(w http.ResponseWriter, r *http.Request) {
	ws, err := c.backend.GetWorkspace(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jobs := []proto.BackgroundJob{}
	manager := shell.GetBackgroundShellManager()
	for _, id := range manager.List() {
		job, ok := manager.Get(id)
		if !ok || !sameProjectPath(job.WorkspacePath, ws.Path) {
			continue
		}
		if sessionID := r.URL.Query().Get("session_id"); sessionID != "" && job.SessionID != sessionID {
			continue
		}
		stdout, stderr, done, jobErr := job.GetOutput()
		errorText := ""
		if jobErr != nil {
			errorText = jobErr.Error()
		}
		if len(stdout) > 65536 {
			stdout = stdout[len(stdout)-65536:]
		}
		if len(stderr) > 65536 {
			stderr = stderr[len(stderr)-65536:]
		}
		jobs = append(jobs, proto.BackgroundJob{SessionID: job.SessionID, ID: id, Command: job.Command, Description: job.Description, Done: done, Stdout: stdout, Stderr: stderr, Error: errorText})
	}
	jsonEncode(w, jobs)
}

func sameProjectPath(left, right string) bool {
	normalize := func(path string) string {
		if absolute, err := filepath.Abs(path); err == nil {
			path = absolute
		}
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		return filepath.Clean(path)
	}
	left, right = normalize(left), normalize(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

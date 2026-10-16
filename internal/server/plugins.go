package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/SpherePrime/CLI/internal/commands"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/plugins"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/voice"
	"net/http"
	"time"
)

func (c *controllerV1) handleGetPlugins(w http.ResponseWriter, r *http.Request) {
	result, err := c.backend.ListPlugins(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}

func (c *controllerV1) handleGetPluginTasks(w http.ResponseWriter, r *http.Request) {
	result, err := c.backend.PluginTasks(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}

func (c *controllerV1) handleEnablePlugin(w http.ResponseWriter, r *http.Request) {
	if err := c.backend.SetPluginEnabled(r.Context(), r.PathValue("id"), r.PathValue("name"), true); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (c *controllerV1) handleDisablePlugin(w http.ResponseWriter, r *http.Request) {
	if err := c.backend.SetPluginEnabled(r.Context(), r.PathValue("id"), r.PathValue("name"), false); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *controllerV1) handleGetCommands(w http.ResponseWriter, r *http.Request) {
	cfg, err := c.backend.GetWorkspaceConfig(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	result, err := commands.LoadCustomCommands(cfg)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	if result == nil {
		result = []commands.CustomCommand{}
	}
	jsonEncode(w, result)
}

func (c *controllerV1) handleVoiceTranscribe(w http.ResponseWriter, r *http.Request) {
	ws, err := c.backend.GetWorkspace(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	if !plugins.Enabled(ws.Cfg.Config(), "voice") {
		jsonError(w, http.StatusConflict, "voice plugin is disabled")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 28*1024*1024)
	var request proto.VoiceUpload
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid or oversized audio upload")
		return
	}
	if request.MIMEType != "audio/wav" && request.MIMEType != "audio/x-wav" {
		jsonError(w, http.StatusBadRequest, "supported audio format: audio/wav (16 kHz mono 16-bit PCM)")
		return
	}
	wav, err := base64.StdEncoding.DecodeString(request.AudioBase64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid audio base64")
		return
	}
	if _, err := voice.ValidateUpload(wav); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 150*time.Second)
	defer cancel()
	text, err := voice.TranscribeUpload(ctx, plugins.SettingsFor(ws.Cfg.Config(), ws.Cfg.Resolver()), wav)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	jsonEncode(w, proto.VoiceTranscript{Text: text})
}

func (c *controllerV1) handlePostReasoning(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Scope     config.Scope             `json:"scope"`
		ModelType config.SelectedModelType `json:"model_type"`
		Mode      string                   `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, 400, "invalid reasoning request")
		return
	}
	if request.ModelType == "" {
		request.ModelType = config.SelectedModelTypeLarge
	}
	cfg, err := c.backend.GetWorkspaceConfig(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	selected, ok := cfg.Models[request.ModelType]
	if !ok {
		jsonError(w, 400, "selected model not configured")
		return
	}
	model := cfg.GetModel(selected.Provider, selected.Model)
	if model == nil {
		jsonError(w, 400, "model not found")
		return
	}
	mode, err := config.NormalizeModelReasoningMode(*model, request.Mode)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	selected.ReasoningMode = mode
	selected.Think = false
	selected.ReasoningEffort = ""
	if err := c.backend.UpdatePreferredModel(r.PathValue("id"), request.Scope, request.ModelType, selected); err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, selected)
}

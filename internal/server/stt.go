package server

import (
	"encoding/json"
	"net/http"

	"github.com/SpherePrime/CLI/internal/proto"
)

// STT endpoints back the desktop's speech recognition settings: the model
// catalog with install state, background installs with progress, removal,
// and the config write that switches engines.

func (c *controllerV1) handleGetSttCatalog(w http.ResponseWriter, r *http.Request) {
	result, err := c.backend.SttCatalog(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}

func (c *controllerV1) handleSttInstall(w http.ResponseWriter, r *http.Request) {
	var request proto.SttModelRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid STT install request")
		return
	}
	if err := c.backend.StartSttInstall(r.PathValue("id"), request.Provider, request.Model); err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.Started{Started: true})
}

func (c *controllerV1) handleSttRemove(w http.ResponseWriter, r *http.Request) {
	var request proto.SttModelRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid STT remove request")
		return
	}
	if err := c.backend.RemoveSttModel(r.PathValue("id"), request.Provider, request.Model); err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, struct{}{})
}

func (c *controllerV1) handleSttSelect(w http.ResponseWriter, r *http.Request) {
	var request proto.SttModelRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid STT select request")
		return
	}
	result, err := c.backend.SelectSttModel(r.PathValue("id"), request.Provider, request.Model)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}

func (c *controllerV1) handleGetSttTasks(w http.ResponseWriter, r *http.Request) {
	result, err := c.backend.SttTasks(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}

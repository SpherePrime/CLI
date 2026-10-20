package server

import (
	"encoding/json"
	"net/http"

	"github.com/SpherePrime/CLI/internal/proto"
)

func (c *controllerV1) handlePostWorkspaceProviderSave(w http.ResponseWriter, r *http.Request) {
	var request proto.ProviderSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid provider configuration")
		return
	}
	if err := c.backend.SaveProvider(r.PathValue("id"), request); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

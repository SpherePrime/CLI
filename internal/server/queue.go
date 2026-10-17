package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/SpherePrime/CLI/internal/agent"
)

func (c *controllerV1) handleQueueEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := c.backend.QueueEntries(r.PathValue("id"), r.PathValue("sid"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, entries)
}

func (c *controllerV1) handleRunQueuedNow(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil || request.ID == "" {
		http.Error(w, `{"error":"queued prompt id is required"}`, http.StatusBadRequest)
		return
	}
	err := c.backend.RunQueuedNow(r.PathValue("id"), r.PathValue("sid"), request.ID)
	if errors.Is(err, agent.ErrQueuedPromptMissing) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		jsonEncode(w, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

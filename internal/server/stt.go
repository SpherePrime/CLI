package server

import (
	"encoding/json"
	"net/http"

	"github.com/SpherePrime/CLI/internal/proto"
)

func (c *controllerV1) handleGetSttCatalog(w http.ResponseWriter, r *http.Request) {
	result, err := c.backend.SttCatalog(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}

func (c *controllerV1) handleSttSelect(w http.ResponseWriter, r *http.Request) {
	var request proto.SttModelRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	result, err := c.backend.SelectSttModel(r.PathValue("id"), request)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}

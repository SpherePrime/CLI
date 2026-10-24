package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/SpherePrime/CLI/internal/apigen"
	"github.com/SpherePrime/CLI/internal/backend"
)

func (c *controllerV1) agentWorkerEndpoints() []apigen.Endpoint {
	const base = "/v1/workspaces/{id}/agent/sessions/{sid}/workers"
	return []apigen.Endpoint{
		apigen.Get(base).Summary("List session agent team").Tags("agent").PathParam("id", "Workspace ID").PathParam("sid", "Session ID").Fails(400, 404, 409, 500).Handle(c.handleAgentWorkers),
		apigen.Get(base+"/{worker}/messages").Summary("Read worker chat").Tags("agent").PathParam("id", "Workspace ID").PathParam("sid", "Session ID").PathParam("worker", "Worker ID").Fails(400, 404, 409, 500).Handle(c.handleAgentWorkerMessages),
		apigen.Post(base+"/{worker}/messages").Summary("Send worker follow-up").Tags("agent").PathParam("id", "Workspace ID").PathParam("sid", "Session ID").PathParam("worker", "Worker ID").Fails(400, 404, 409, 500).Handle(c.handleAgentWorkerSend),
		apigen.Post(base+"/{worker}/stop").Summary("Stop worker").Tags("agent").PathParam("id", "Workspace ID").PathParam("sid", "Session ID").PathParam("worker", "Worker ID").Fails(400, 404, 409, 500).Handle(c.handleAgentWorkerStop),
	}
}

func (c *controllerV1) workerError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, backend.ErrAgentWorkerNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, backend.ErrAgentWorkersUnsupported):
		jsonError(w, http.StatusConflict, err.Error())
	case errors.Is(err, backend.ErrAgentWorkerInput):
		jsonError(w, http.StatusBadRequest, err.Error())
	default:
		c.handleError(w, r, err)
	}
}

func (c *controllerV1) handleAgentWorkers(w http.ResponseWriter, r *http.Request) {
	workers, err := c.backend.GetAgentWorkers(r.Context(), r.PathValue("id"), r.PathValue("sid"))
	if err != nil {
		c.workerError(w, r, err)
		return
	}
	jsonEncode(w, map[string]any{"workers": workers})
}

func (c *controllerV1) handleAgentWorkerMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := c.backend.ListAgentWorkerMessages(r.Context(), r.PathValue("id"), r.PathValue("sid"), r.PathValue("worker"))
	if err != nil {
		c.workerError(w, r, err)
		return
	}
	jsonEncode(w, map[string]any{"messages": messagesToProto(messages)})
}

func (c *controllerV1) handleAgentWorkerSend(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 132096)).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid message")
		return
	}
	worker, err := c.backend.SendAgentWorkerMessage(r.Context(), r.PathValue("id"), r.PathValue("sid"), r.PathValue("worker"), request.Text)
	if err != nil {
		c.workerError(w, r, err)
		return
	}
	jsonEncode(w, worker)
}

func (c *controllerV1) handleAgentWorkerStop(w http.ResponseWriter, r *http.Request) {
	err := c.backend.StopAgentWorker(r.Context(), r.PathValue("id"), r.PathValue("sid"), r.PathValue("worker"))
	if err != nil {
		c.workerError(w, r, err)
		return
	}
	jsonEncode(w, map[string]bool{"success": true})
}

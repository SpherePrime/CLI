package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/SpherePrime/CLI/internal/goal"
	"github.com/SpherePrime/CLI/internal/proto"
)

func goalToProto(current goal.Goal) proto.ChatGoal {
	return proto.ChatGoal{
		SessionID: current.SessionID,
		Text:      current.Text,
		Status:    string(current.Status),
		Summary:   current.Summary,
		CreatedAt: current.CreatedAt,
		UpdatedAt: current.UpdatedAt,
	}
}

func (c *controllerV1) handleGetSessionGoal(w http.ResponseWriter, r *http.Request) {
	current, found, err := c.backend.GetSessionGoal(r.Context(), r.PathValue("id"), r.PathValue("sid"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	if !found {
		jsonEncode(w, nil)
		return
	}
	jsonEncode(w, goalToProto(current))
}

func (c *controllerV1) handlePutSessionGoal(w http.ResponseWriter, r *http.Request) {
	var request proto.ChatGoalRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}
	saved, err := c.backend.SetSessionGoal(r.Context(), r.PathValue("id"), r.PathValue("sid"), request.Text)
	if errors.Is(err, goal.ErrEmptyText) {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, goalToProto(saved))
}

func (c *controllerV1) handleDeleteSessionGoal(w http.ResponseWriter, r *http.Request) {
	if err := c.backend.ClearSessionGoal(r.Context(), r.PathValue("id"), r.PathValue("sid")); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

package acp

import (
	"context"
	"log/slog"

	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/proto"
)

const (
	PermissionOptionAllowOnce   = "allow_once"
	PermissionOptionAllowAlways = "allow_always"
	PermissionOptionRejectOnce  = "reject_once"
)

func defaultPermissionOptions() []PermissionOption {
	return []PermissionOption{
		{
			OptionID: PermissionOptionAllowOnce,
			Name:     "Allow once",
			Kind:     PermissionKindAllowOnce,
		},
		{
			OptionID: PermissionOptionAllowAlways,
			Name:     "Allow for this session",
			Kind:     PermissionKindAllowAlways,
		},
		{
			OptionID: PermissionOptionRejectOnce,
			Name:     "Reject",
			Kind:     PermissionKindRejectOnce,
		},
	}
}

func (s *Server) handlePermissionRequest(
	runCtx context.Context,
	run *activePromptRun,
	sess *sessionState,
	perm permission.PermissionRequest,
) {
	permCtx, cancel := context.WithCancel(runCtx)
	permKey := perm.ID
	if permKey == "" {
		permKey = perm.ToolCallID
	}
	run.addPermission(permKey, cancel)
	defer func() {
		run.removePermission(permKey)
		cancel()
	}()

	reqParams := RequestPermissionParams{
		SessionID: perm.SessionID,
		ToolCall:  buildPermissionToolCall(perm),
		Options:   defaultPermissionOptions(),
	}

	var res RequestPermissionResult
	err := s.transport.Call(permCtx, "session/request_permission", reqParams, &res)

	action := resolvePermissionAction(res, err)
	if _, grantErr := s.backend.GrantPermission(sess.workspaceID, proto.PermissionGrant{
		Permission: permissionToProto(perm),
		Action:     action,
	}); grantErr != nil {
		slog.Warn("Failed to resolve ACP permission request", "permission_id", perm.ID, "error", grantErr)
	}
}

func resolvePermissionAction(res RequestPermissionResult, err error) proto.PermissionAction {
	if err != nil {
		return proto.PermissionDeny
	}
	if res.Outcome.Outcome != PermissionOutcomeSelected {
		return proto.PermissionDeny
	}
	switch res.Outcome.OptionID {
	case PermissionOptionAllowOnce:
		return proto.PermissionAllow
	case PermissionOptionAllowAlways:
		return proto.PermissionAllowForSession
	default:
		return proto.PermissionDeny
	}
}

func buildPermissionToolCall(perm permission.PermissionRequest) PermissionToolCall {
	toolCallID := perm.ToolCallID
	if toolCallID == "" {
		toolCallID = perm.ID
	}
	title := perm.Description
	if title == "" {
		if perm.ToolName != "" && perm.Action != "" {
			title = perm.ToolName + ": " + perm.Action
		} else if perm.ToolName != "" {
			title = perm.ToolName
		} else {
			title = "tool permission"
		}
	}
	var locations []ToolCallLocation
	if perm.Path != "" {
		locations = []ToolCallLocation{{Path: perm.Path}}
	}
	return PermissionToolCall{
		ToolCallID: toolCallID,
		Title:      title,
		Kind:       MapToolKind(perm.ToolName),
		Status:     ToolStatusPending,
		Locations:  locations,
		RawInput:   perm.Params,
	}
}

func permissionToProto(perm permission.PermissionRequest) proto.PermissionRequest {
	return proto.PermissionRequest{
		ID:          perm.ID,
		SessionID:   perm.SessionID,
		ToolCallID:  perm.ToolCallID,
		ToolName:    perm.ToolName,
		Description: perm.Description,
		Action:      perm.Action,
		Params:      perm.Params,
		Path:        perm.Path,
	}
}

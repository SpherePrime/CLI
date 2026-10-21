package backend

import (
	"fmt"

	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/proto"
)

// GrantPermission grants, denies, or persistently grants a permission
// request. The returned bool reports whether this call resolved the
// pending request (true) or found it already resolved by a previous
// caller (false). A false return is not an error.
func (b *Backend) GrantPermission(workspaceID string, req proto.PermissionGrant) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}

	perm := permission.PermissionRequest{
		ID:          req.Permission.ID,
		SessionID:   req.Permission.SessionID,
		ToolCallID:  req.Permission.ToolCallID,
		ToolName:    req.Permission.ToolName,
		Description: req.Permission.Description,
		Action:      req.Permission.Action,
		Params:      req.Permission.Params,
		Path:        req.Permission.Path,
	}

	switch req.Action {
	case proto.PermissionAllow:
		return ws.Permissions.Grant(perm), nil
	case proto.PermissionAllowForSession:
		return ws.Permissions.GrantPersistent(perm), nil
	case proto.PermissionDeny:
		return ws.Permissions.Deny(perm), nil
	default:
		return false, ErrInvalidPermissionAction
	}
}

// SetPermissionsSkip sets whether permission prompts are skipped.
func (b *Backend) SetPermissionsSkip(workspaceID string, skip bool, sessionIDs ...string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	if len(sessionIDs) > 0 && sessionIDs[0] != "" {
		scoped, ok := ws.Permissions.(permission.SessionSkipService)
		if !ok {
			return fmt.Errorf("session permissions are unavailable")
		}
		scoped.SetSessionSkipRequests(sessionIDs[0], skip)
	} else {
		ws.Permissions.SetSkipRequests(skip)
	}
	return nil
}

// GetPermissionsSkip returns whether permission prompts are skipped.
func (b *Backend) GetPermissionsSkip(workspaceID string, sessionIDs ...string) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}

	if len(sessionIDs) > 0 && sessionIDs[0] != "" {
		scoped, ok := ws.Permissions.(permission.SessionSkipService)
		if !ok {
			return false, fmt.Errorf("session permissions are unavailable")
		}
		return scoped.SessionSkipRequests(sessionIDs[0]), nil
	}
	return ws.Permissions.SkipRequests(), nil
}

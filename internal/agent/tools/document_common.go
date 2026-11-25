package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/fsext"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const documentCitePathNote = "Always include the exact file path in your final reply to the user."

type DocumentPermissionsParams struct {
	Action     string `json:"action"`
	OutputPath string `json:"output_path"`
	InputPath  string `json:"input_path,omitempty"`
}

func firstTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func requestDocumentWritePermission(
	ctx context.Context,
	permissions permission.Service,
	call fantasy.ToolCall,
	workingDir string,
	toolName string,
	action string,
	targetPath string,
	inputPath string,
	description string,
) (bool, error) {
	if permissions == nil {
		return false, fmt.Errorf("permission service is required")
	}
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return false, fmt.Errorf("session ID is required")
	}
	return permissions.Request(ctx, permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        fsext.PathOrPrefix(targetPath, workingDir),
		ToolCallID:  call.ID,
		ToolName:    toolName,
		Action:      "write",
		Description: description,
		Params: DocumentPermissionsParams{
			Action:     action,
			OutputPath: targetPath,
			InputPath:  inputPath,
		},
	})
}

func documentJSONResponse(payload any) (fantasy.ToolResponse, error) {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("encode tool response: %w", err)
	}
	return fantasy.NewTextResponse(string(encoded)), nil
}

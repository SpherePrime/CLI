package tools

import (
	"encoding/json"
	"fmt"
)

func (m *Tool) permissionDescription(input string) string {
	if m.tool.Name == "window_request_access" {
		var target struct {
			Title string `json:"title"`
			HWND  string `json:"hwnd"`
			PID   int    `json:"pid"`
			Mode  string `json:"mode"`
		}
		if json.Unmarshal([]byte(input), &target) == nil && target.Title != "" && target.HWND != "" && target.PID > 0 && (target.Mode == "view" || target.Mode == "control") {
			return fmt.Sprintf("Allow %s of window %q (PID %d, HWND %s) in this chat", target.Mode, target.Title, target.PID, target.HWND)
		}
	}
	return fmt.Sprintf("execute %s with the following parameters:", m.Info().Name)
}

package tools

import (
	"encoding/json"
	"fmt"
)

type windowPermissionTarget struct {
	Title string `json:"title"`
	HWND  string `json:"hwnd"`
	PID   int    `json:"pid"`
	Mode  string `json:"mode"`
}

func (m *Tool) permissionDescription(input string) string {
	if description, ok := m.customPermissionDescription(input); ok {
		return description
	}
	return fmt.Sprintf("execute %s with the following parameters:", m.Info().Name)
}

func (m *Tool) customPermissionDescription(input string) (string, bool) {
	switch m.tool.Name {
	case "window_request_access":
		return windowRequestAccessDescription(input)
	case "app_attach":
		return appAttachDescription(input)
	}
	return "", false
}

func windowRequestAccessDescription(input string) (string, bool) {
	target, ok := parseWindowPermissionTarget(input)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("Allow %s of window %q (PID %d, HWND %s) in this chat", target.Mode, target.Title, target.PID, target.HWND), true
}

func appAttachDescription(input string) (string, bool) {
	target, ok := parseWindowPermissionTarget(input)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("Allow %s of application window %q (PID %d, HWND %s) in this chat", target.Mode, target.Title, target.PID, target.HWND), true
}

func parseWindowPermissionTarget(input string) (windowPermissionTarget, bool) {
	var target windowPermissionTarget
	if json.Unmarshal([]byte(input), &target) != nil {
		return windowPermissionTarget{}, false
	}
	if target.Title == "" || target.HWND == "" || target.PID <= 0 || (target.Mode != "view" && target.Mode != "control") {
		return windowPermissionTarget{}, false
	}
	return target, true
}

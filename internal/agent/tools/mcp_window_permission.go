package tools

import (
	"encoding/json"
	"fmt"
	"strings"
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
	case "app_launch":
		return appLaunchDescription(input)
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
	var target windowPermissionTarget
	if json.Unmarshal([]byte(input), &target) != nil {
		return "", false
	}
	mode := target.Mode
	if mode == "" {
		mode = "control"
	}
	if mode != "view" && mode != "control" {
		return "", false
	}
	var details []string
	if target.PID > 0 {
		details = append(details, fmt.Sprintf("PID %d", target.PID))
	}
	if target.HWND != "" {
		details = append(details, "HWND "+target.HWND)
	}
	switch {
	case target.Title != "" && len(details) > 0:
		return fmt.Sprintf("Allow %s of application window %q (%s) in this chat", mode, target.Title, strings.Join(details, ", ")), true
	case target.Title != "":
		return fmt.Sprintf("Allow %s of application window matching %q in this chat", mode, target.Title), true
	case len(details) > 0:
		return fmt.Sprintf("Allow %s of application window %s in this chat", mode, strings.Join(details, ", ")), true
	}
	return "", false
}

func appLaunchDescription(input string) (string, bool) {
	var request struct {
		Command string `json:"command"`
		Cwd     string `json:"cwd"`
	}
	if json.Unmarshal([]byte(input), &request) != nil || strings.TrimSpace(request.Command) == "" {
		return "", false
	}
	location := "the project root"
	if request.Cwd != "" {
		location = request.Cwd
	}
	return fmt.Sprintf("Run %q in %s and control the application window it opens in this chat", request.Command, location), true
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

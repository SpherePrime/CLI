package shellconfig

import (
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

func optionVoice(options map[string]any, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: option voice <key> [value]")
	}

	key := args[2]
	voice := childMap(options, "voice")

	if key == "on" || key == "off" {
		voice["enabled"] = key == "on"
		slog.Info("Voice option set in shell config", "key", "enabled", "value", key == "on")
		return nil
	}

	value := ""
	if len(args) > 3 {
		value = strings.TrimSpace(strings.Join(args[3:], " "))
	}

	switch key {
	case "enabled":
		parsed, err := parseBool(value)
		if err != nil {
			return usage(stderr, fmt.Sprintf("option voice enabled expects true/false, got %q", value))
		}
		voice["enabled"] = parsed
	case "language", "record-command":
		if value == "" {
			return usage(stderr, fmt.Sprintf("option voice %s requires a value", key))
		}
		voice[voiceJSONKey(key)] = value
	case "max-duration":
		if value == "" {
			return usage(stderr, "option voice max-duration requires a value")
		}
		seconds, err := strconv.Atoi(value)
		if err != nil || seconds <= 0 {
			return usage(stderr, fmt.Sprintf("option voice max-duration expects a positive number of seconds, got %q", value))
		}
		voice["max_duration"] = seconds
	default:
		return usage(stderr, fmt.Sprintf("option voice: unknown key %q", key))
	}

	slog.Info("Voice option set in shell config", "key", key)
	return nil
}

func voiceJSONKey(key string) string {
	if key == "record-command" {
		return "record_command"
	}
	return key
}

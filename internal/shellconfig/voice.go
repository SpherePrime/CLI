package shellconfig

import (
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

// optionVoice implements "option voice <key> [value]" for the nested
// options.voice block, which configures microphone dictation.
//
// Values are stored as written: env-var references in api-key stay unresolved
// here and are expanded when the voice pipeline reads them.
//
// Examples:
//
//	option voice on
//	option voice language ru
//	option voice model /models/ggml-small.bin
//	option voice base-url http://localhost:8000
//	option voice record-command "sox -t alsa default -t raw -r 16000 -b 16 -c 1 -"
func optionVoice(options map[string]any, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: option voice <key> [value]")
	}

	// args is ["option", "voice", <key>, <value>], matching "option ui ...".
	key := args[2]
	voice := childMap(options, "voice")

	// "enabled" is the one boolean, and it is the common case, so it also
	// accepts the bare "option voice on" and "option voice off" spellings.
	if key == "on" || key == "off" {
		voice["enabled"] = key == "on"
		slog.Info("Voice option set in shell config", "key", "enabled", "value", key == "on")
		return nil
	}

	// A value may arrive as several words when it was not quoted. Joining
	// them keeps commands like record-command working the way they read.
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
	case "engine", "model", "language", "base-url", "api-key", "record-command", "transcribe-command":
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

// voiceJSONKey maps the kebab-case option key onto its JSON field.
func voiceJSONKey(key string) string {
	switch key {
	case "base-url":
		return "base_url"
	case "api-key":
		return "api_key"
	case "record-command":
		return "record_command"
	case "transcribe-command":
		return "transcribe_command"
	}
	return key
}

package voice

import "strings"

var AllowedLanguages = []string{"auto", "ru", "en", "uk"}

func NormalizeLanguage(language string) string {
	trimmed := strings.ToLower(strings.TrimSpace(language))
	if trimmed == "" || trimmed == "auto" {
		return ""
	}
	if cut := strings.IndexAny(trimmed, ",;-_."); cut >= 0 {
		trimmed = trimmed[:cut]
	}
	switch trimmed {
	case "ru", "rus", "russian":
		return "ru"
	case "en", "eng", "english":
		return "en"
	case "uk", "ukr", "ua", "ukrainian":
		return "uk"
	default:
		return ""
	}
}

func DisplayLanguage(language string) string {
	normalized := NormalizeLanguage(language)
	if normalized == "" {
		return "auto"
	}
	return normalized
}

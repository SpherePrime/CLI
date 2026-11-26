package voice

import "strings"

const DefaultLanguage = "ru"

var AllowedLanguages = []string{"ru", "en", "uk", "auto"}

func NormalizeLanguage(language string) string {
	trimmed := strings.ToLower(strings.TrimSpace(language))
	if trimmed == "" {
		return DefaultLanguage
	}
	if trimmed == "auto" {
		return "auto"
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
		return DefaultLanguage
	}
}

func DisplayLanguage(language string) string {
	return NormalizeLanguage(language)
}

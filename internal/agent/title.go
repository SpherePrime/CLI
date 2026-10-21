package agent

import "strings"

func IsDefaultSessionTitle(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "", "untitled session", "new session", "new chat", "новый чат":
		return true
	default:
		return false
	}
}

func promptSessionTitle(prompt string) string {
	prompt, _, _ = strings.Cut(prompt, "<working-context>")
	title := strings.Join(strings.Fields(prompt), " ")
	runes := []rune(title)
	if len(runes) > 50 {
		title = string(runes[:49]) + "…"
	}
	if title == "" {
		return DefaultSessionName
	}
	return title
}

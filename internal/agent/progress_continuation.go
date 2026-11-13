package agent

import "strings"

const progressContinuationPrompt = "Continue executing the user's task now. Your previous message only announced further work. Use the available tools instead of ending with another promise. If blocked, state the specific blocker or ask the necessary question."

func unfinishedProgress(text string, incomplete bool) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" || len([]rune(text)) > 1800 {
		return false
	}
	for _, blocker := range []string{"?", "нужен пароль", "нужен доступ", "нет доступа", "не могу", "заблокирован", "permission", "blocked", "cannot", "waiting for", "ожидаю", "в фоне", "background"} {
		if strings.Contains(text, blocker) {
			return false
		}
	}
	for _, phrase := range []string{"сейчас разберусь", "сейчас проверю", "дальше анализирую", "продолжаю проверку", "i'll investigate", "i will investigate"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return incomplete && (strings.Contains(text, "проверю, как") || strings.Contains(text, "проверю как"))
}

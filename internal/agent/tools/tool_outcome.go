package tools

import (
	"github.com/SpherePrime/CLI/internal/shell"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

func shellResponse(text string, err error, done bool) fantasy.ToolResponse {
	response := fantasy.NewTextResponse(text)
	switch {
	case err != nil || shell.IsInterrupt(err):
		response.Outcome = fantasy.ToolOutcomeFailure
		response.IsError = true
	case !done:
		response.Outcome = fantasy.ToolOutcomeRunning
	default:
		response.Outcome = fantasy.ToolOutcomeSuccess
	}
	return response
}

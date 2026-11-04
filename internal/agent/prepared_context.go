package agent

import (
	"encoding/json"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

type preparedContextView struct {
	Definitions []fantasy.Tool
	Messages    []fantasy.Message
}

func (a *sessionAgent) recordPreparedContext(sessionID string, definitions []fantasy.Tool, messages []fantasy.Message) {
	if a.preparedViews == nil || sessionID == "" {
		return
	}
	a.preparedViews.Set(sessionID, preparedContextView{
		Definitions: append([]fantasy.Tool(nil), definitions...),
		Messages:    cloneFantasyMessages(messages),
	})
}

func (a *sessionAgent) preparedContext(sessionID string) (preparedContextView, bool) {
	if a.preparedViews == nil {
		return preparedContextView{}, false
	}
	return a.preparedViews.Get(sessionID)
}

func addPreparedToolDefinitions(view preparedContextView, add func(string, int64)) {
	for _, definition := range view.Definitions {
		function, ok := definition.(fantasy.FunctionTool)
		if !ok {
			continue
		}
		key := ContextTools
		if strings.HasPrefix(function.Name, "mcp_") {
			key = ContextMCPTools
		}
		tokens := approxTokenCount(function.Name) + approxTokenCount(function.Description)
		if schema, err := json.Marshal(function.InputSchema); err == nil {
			tokens += approxTokenCount(string(schema))
		}
		add(key, tokens)
	}
}

func addPreparedMessages(view preparedContextView, add func(string, int64)) {
	for _, message := range view.Messages {
		addFantasyMessageParts(message, add)
	}
}

func addFantasyMessageParts(message fantasy.Message, add func(string, int64)) {
	if message.Role == fantasy.MessageRoleSystem {
		return
	}
	textKey := ContextAssistant
	if message.Role == fantasy.MessageRoleUser {
		textKey = ContextUserMessages
	}
	for _, part := range message.Content {
		switch p := part.(type) {
		case fantasy.TextPart:
			add(textKey, approxTokenCount(p.Text))
		case fantasy.ReasoningPart:
			add(ContextReasoning, approxTokenCount(p.Text))
		case fantasy.FilePart:
			add(ContextAttachments, estimateMediaTokens(p.MediaType, p.Filename, len(p.Data)))
		case fantasy.ToolCallPart:
			add(ContextToolCalls, estimateToolCallTokens(p.ToolName, p.Input))
		case fantasy.ToolResultPart:
			addFantasyToolResult(p, add)
		}
	}
}

func addFantasyToolResult(result fantasy.ToolResultPart, add func(string, int64)) {
	switch output := result.Output.(type) {
	case fantasy.ToolResultOutputContentText:
		add(ContextToolResults, approxTokenCount(output.Text))
	case fantasy.ToolResultOutputContentError:
		if output.Error != nil {
			add(ContextToolResults, approxTokenCount(output.Error.Error()))
		}
	case fantasy.ToolResultOutputContentMedia:
		add(ContextToolResults, approxTokenCount(output.Text))
		add(ContextAttachments, estimateMediaTokens(output.MediaType, "", len(output.Data)))
	}
	add(ContextToolResults, approxTokenCount(result.ClientMetadata))
}

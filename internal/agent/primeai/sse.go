package primeai

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	sseInitialBufSize = 64 * 1024
	sseMaxBufSize     = 8 * 1024 * 1024
)

func buildCompletionRequest(modelID string, call fantasy.Call) RouterCompletionRequest {
	var systemParts []string
	var messages []RouterMessage

	for i, msg := range call.Prompt {
		switch msg.Role {
		case fantasy.MessageRoleSystem:
			text := collectTextParts(msg.Content)
			if text == "" {
				continue
			}
			if i == 0 || len(messages) == 0 {
				systemParts = append(systemParts, text)
			} else {
				messages = append(messages, RouterMessage{
					Role:    "system",
					Content: text,
				})
			}

		case fantasy.MessageRoleUser:
			messages = append(messages, convertUserMessage(msg))

		case fantasy.MessageRoleAssistant:
			text := collectTextParts(msg.Content)
			var toolCalls []RouterToolCall
			for _, part := range msg.Content {
				if part.GetType() != fantasy.ContentTypeToolCall {
					continue
				}
				tc, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](part)
				if !ok {
					continue
				}
				args := tc.Input
				if strings.TrimSpace(args) == "" {
					args = "{}"
				}
				toolCalls = append(toolCalls, RouterToolCall{
					ID:            tc.ToolCallID,
					Name:          tc.ToolName,
					ArgumentsJson: args,
				})
			}
			var content any = text
			if text == "" && len(toolCalls) > 0 {
				content = nil
			}
			messages = append(messages, RouterMessage{
				Role:      "assistant",
				Content:   content,
				ToolCalls: toolCalls,
			})

		case fantasy.MessageRoleTool:
			for _, part := range msg.Content {
				if part.GetType() != fantasy.ContentTypeToolResult {
					continue
				}
				tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
				if !ok {
					continue
				}
				messages = append(messages, RouterMessage{
					Role:       "tool",
					Content:    formatToolResultOutput(tr.Output),
					ToolCallID: tr.ToolCallID,
				})
			}
		}
	}

	var tools []RouterTool
	for _, t := range call.Tools {
		ft, ok := t.(fantasy.FunctionTool)
		if !ok {
			continue
		}
		params := ft.InputSchema
		if params == nil {
			params = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
		tools = append(tools, RouterTool{
			Name:        ft.Name,
			Description: ft.Description,
			Parameters:  params,
		})
	}

	req := RouterCompletionRequest{
		Model:        cmp.Or(modelID, AutomaticModelID),
		Messages:     messages,
		SystemPrompt: strings.Join(systemParts, "\n\n"),
		Tools:        tools,
		Temperature:  call.Temperature,
	}
	if call.MaxOutputTokens != nil && *call.MaxOutputTokens > 0 {
		v := *call.MaxOutputTokens
		if v > 1_000_000 {
			v = 1_000_000
		}
		req.MaxTokens = &v
	}

	if optRaw, ok := call.ProviderOptions[Name]; ok {
		if opts, ok := optRaw.(*Options); ok && opts != nil {
			req.ReasoningEffort = opts.ReasoningEffort
			req.ShowReasoning = opts.ShowReasoning
		}
	}

	return req
}

func collectTextParts(parts []fantasy.MessagePart) string {
	var b strings.Builder
	for _, part := range parts {
		if part.GetType() != fantasy.ContentTypeText {
			continue
		}
		tp, ok := fantasy.AsMessagePart[fantasy.TextPart](part)
		if !ok {
			continue
		}
		b.WriteString(tp.Text)
	}
	return b.String()
}

func convertUserMessage(msg fantasy.Message) RouterMessage {
	var contentParts []RouterContentPart
	hasImage := false
	var textOnly strings.Builder

	for _, part := range msg.Content {
		switch part.GetType() {
		case fantasy.ContentTypeText:
			tp, ok := fantasy.AsMessagePart[fantasy.TextPart](part)
			if !ok {
				continue
			}
			textOnly.WriteString(tp.Text)
			contentParts = append(contentParts, RouterContentPart{
				Type: "text",
				Text: tp.Text,
			})
		case fantasy.ContentTypeFile:
			fp, ok := fantasy.AsMessagePart[fantasy.FilePart](part)
			if !ok {
				continue
			}
			if isSupportedImageMIME(fp.MediaType) && len(fp.Data) > 0 {
				hasImage = true
				dataURL := fmt.Sprintf("data:%s;base64,%s", fp.MediaType, base64.StdEncoding.EncodeToString(fp.Data))
				contentParts = append(contentParts, RouterContentPart{
					Type:     "image_url",
					ImageURL: &RouterImageURL{URL: dataURL},
				})
			}
		}
	}

	if hasImage {
		return RouterMessage{
			Role:    "user",
			Content: contentParts,
		}
	}
	return RouterMessage{
		Role:    "user",
		Content: textOnly.String(),
	}
}

func isSupportedImageMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func formatToolResultOutput(output fantasy.ToolResultOutputContent) string {
	if output == nil {
		return ""
	}
	switch output.GetType() {
	case fantasy.ToolResultContentTypeText:
		if v, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](output); ok {
			return v.Text
		}
	case fantasy.ToolResultContentTypeError:
		if v, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](output); ok && v.Error != nil {
			return v.Error.Error()
		}
	case fantasy.ToolResultContentTypeMedia:
		if v, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](output); ok {
			if v.Text != "" {
				return v.Text
			}
			return "[media]"
		}
	}
	return ""
}

func (m *languageModel) openCompletionStream(ctx context.Context, call fantasy.Call) (*http.Response, error) {
	reqPayload := buildCompletionRequest(m.modelID, call)
	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal PrimeAI completion request: %w", err)
	}

	endpoint := m.client.base() + "/api/plugin/chat/completions"
	resp, err := m.client.doWithSession(ctx, func(reqCtx context.Context) (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if reqErr != nil {
			return nil, reqErr
		}
		for k, v := range call.Headers {
			if strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Content-Type") || strings.EqualFold(k, "Accept") {
				continue
			}
			req.Header.Set(k, v)
		}
		if call.UserAgent != "" {
			req.Header.Set("User-Agent", call.UserAgent)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		return req, nil
	})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		rawErr, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, m.parseHTTPError(resp.StatusCode, endpoint, rawErr)
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(ct, "text/event-stream") {
		_ = resp.Body.Close()
		return nil, &fantasy.ProviderError{
			URL:     endpoint,
			Title:   "Invalid Response",
			Message: "PrimeAI did not return an event stream",
		}
	}

	return resp, nil
}

func (m *languageModel) parseHTTPError(statusCode int, endpoint string, raw []byte) error {
	var payload struct {
		Code           string          `json:"code"`
		Error          string          `json:"error"`
		Window         string          `json:"window"`
		ResetInSeconds int64           `json:"resetInSeconds"`
		Limits         json.RawMessage `json:"limits"`
	}
	_ = json.Unmarshal(raw, &payload)

	if len(payload.Limits) > 0 && m.client.limits != nil {
		if l, err := DecodeLimits(payload.Limits); err == nil {
			m.client.limits.Set(l)
		}
	}

	title := cmp.Or(payload.Code, "PrimeAI Error")
	msg := cmp.Or(payload.Error, payload.Code, fmt.Sprintf("HTTP %d", statusCode))
	if statusCode == http.StatusTooManyRequests && payload.Code == "PLUGIN_LIMIT_EXCEEDED" && payload.Error == "" {
		msg = fmt.Sprintf("PrimeAI %s limit exceeded (reset in %ds)", cmp.Or(payload.Window, "tariff"), payload.ResetInSeconds)
	}
	if statusCode == http.StatusServiceUnavailable && payload.Code == "MAINTENANCE_MODE" && payload.Error == "" {
		msg = "PrimeAI is temporarily in maintenance mode"
	}

	return &fantasy.ProviderError{
		URL:            endpoint,
		StatusCode:     statusCode,
		Title:          title,
		Message:        msg,
		AuthError:      statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden,
		TransientError: statusCode == http.StatusInternalServerError || statusCode == http.StatusBadGateway || statusCode == http.StatusServiceUnavailable || statusCode == http.StatusGatewayTimeout,
	}
}

func (m *languageModel) consumeSSE(ctx context.Context, body io.ReadCloser, yield func(fantasy.StreamPart) bool) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, sseInitialBufSize), sseMaxBufSize)

	var (
		eventName       string
		dataLines       []string
		activeText      bool
		activeReasoning bool
		hasToolCalls    bool
		completed       bool
		usage           fantasy.Usage
	)

	closeOpenBlocks := func() bool {
		if activeReasoning {
			activeReasoning = false
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeReasoningEnd, ID: "r0"}) {
				return false
			}
		}
		if activeText {
			activeText = false
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "0"}) {
				return false
			}
		}
		return true
	}

	dispatchFrame := func() bool {
		if eventName == "" || len(dataLines) == 0 {
			eventName = ""
			dataLines = dataLines[:0]
			return true
		}
		name := eventName
		rawData := strings.Join(dataLines, "\n")
		eventName = ""
		dataLines = dataLines[:0]

		switch name {
		case "route":
			var ev sseRouteEvent
			if err := json.Unmarshal([]byte(rawData), &ev); err != nil {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: fmt.Errorf("invalid route event: %w", err)})
				return false
			}
			return true

		case "thinking":
			var ev sseDeltaEvent
			if err := json.Unmarshal([]byte(rawData), &ev); err != nil {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: fmt.Errorf("invalid thinking event: %w", err)})
				return false
			}
			if ev.Delta == "" {
				return true
			}
			if activeText {
				activeText = false
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "0"}) {
					return false
				}
			}
			if !activeReasoning {
				activeReasoning = true
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeReasoningStart, ID: "r0"}) {
					return false
				}
			}
			return yield(fantasy.StreamPart{
				Type:  fantasy.StreamPartTypeReasoningDelta,
				ID:    "r0",
				Delta: ev.Delta,
			})

		case "text":
			var ev sseDeltaEvent
			if err := json.Unmarshal([]byte(rawData), &ev); err != nil {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: fmt.Errorf("invalid text event: %w", err)})
				return false
			}
			if ev.Delta == "" {
				return true
			}
			if activeReasoning {
				activeReasoning = false
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeReasoningEnd, ID: "r0"}) {
					return false
				}
			}
			if !activeText {
				activeText = true
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "0"}) {
					return false
				}
			}
			return yield(fantasy.StreamPart{
				Type:  fantasy.StreamPartTypeTextDelta,
				ID:    "0",
				Delta: ev.Delta,
			})

		case "tool_call":
			var ev sseToolCallEvent
			if err := json.Unmarshal([]byte(rawData), &ev); err != nil {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: fmt.Errorf("invalid tool_call event: %w", err)})
				return false
			}
			if !closeOpenBlocks() {
				return false
			}
			args := ev.Call.ArgumentsJson
			if strings.TrimSpace(args) == "" {
				args = "{}"
			}
			hasToolCalls = true
			if !yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeToolInputStart,
				ID:           ev.Call.ID,
				ToolCallName: ev.Call.Name,
			}) {
				return false
			}
			if !yield(fantasy.StreamPart{
				Type:  fantasy.StreamPartTypeToolInputDelta,
				ID:    ev.Call.ID,
				Delta: args,
			}) {
				return false
			}
			if !yield(fantasy.StreamPart{
				Type: fantasy.StreamPartTypeToolInputEnd,
				ID:   ev.Call.ID,
			}) {
				return false
			}
			return yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            ev.Call.ID,
				ToolCallName:  ev.Call.Name,
				ToolCallInput: args,
			})

		case "usage":
			var ev sseUsageEvent
			if err := json.Unmarshal([]byte(rawData), &ev); err != nil {
				return true
			}
			usage.InputTokens = ev.InputTokens
			usage.OutputTokens = ev.OutputTokens
			usage.CacheReadTokens = ev.CachedInputTokens
			usage.TotalTokens = ev.InputTokens + ev.OutputTokens
			return true

		case "limits":
			if l, err := DecodeLimits([]byte(rawData)); err == nil && m.client.limits != nil {
				m.client.limits.Set(l)
			}
			return true

		case "error":
			var ev sseErrorEvent
			if err := json.Unmarshal([]byte(rawData), &ev); err != nil {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: fmt.Errorf("invalid error event: %w", err)})
				return false
			}
			_ = closeOpenBlocks()
			yield(fantasy.StreamPart{
				Type: fantasy.StreamPartTypeError,
				Error: &fantasy.ProviderError{
					Title:          cmp.Or(ev.Code, "PrimeAI Error"),
					Message:        cmp.Or(ev.Detail, ev.Code, "PrimeAI stream error"),
					TransientError: ev.Retryable,
				},
			})
			return false

		case "done":
			completed = true
			if !closeOpenBlocks() {
				return false
			}
			finishReason := fantasy.FinishReasonStop
			if hasToolCalls {
				finishReason = fantasy.FinishReasonToolCalls
			}
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: finishReason,
				Usage:        usage,
			})
			return false
		}

		return true
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if !dispatchFrame() {
				return
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimPrefix(data, " ")
			dataLines = append(dataLines, data)
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: ctxErr})
			return
		}
		yield(fantasy.StreamPart{
			Type: fantasy.StreamPartTypeError,
			Error: &fantasy.ProviderError{
				Title:          "Stream Error",
				Message:        err.Error(),
				Cause:          err,
				TransientError: true,
			},
		})
		return
	}

	if eventName != "" && len(dataLines) > 0 {
		if !dispatchFrame() && completed {
			return
		}
	}

	if !completed {
		if ctxErr := ctx.Err(); ctxErr != nil {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: ctxErr})
			return
		}
		yield(fantasy.StreamPart{
			Type:  fantasy.StreamPartTypeError,
			Error: fantasy.NewIncompleteStreamError(),
		})
	}
}

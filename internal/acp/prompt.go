package acp

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/internal/session"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
)

const defaultCancelWaitTimeout = 500 * time.Millisecond

func (s *Server) handlePrompt(ctx context.Context, params PromptParams) (PromptResult, error) {
	if err := s.EnsureAuthenticated(ctx); err != nil {
		return PromptResult{}, err
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return PromptResult{}, NewRPCError(CodeInvalidParams, "sessionId is required")
	}
	sess, ok := s.getSession(params.SessionID)
	if !ok {
		return PromptResult{}, NewRPCError(CodeResourceNotFound, fmt.Sprintf("session not found: %s", params.SessionID))
	}

	promptText, attachments, err := convertPromptBlocks(params.Prompt)
	if err != nil {
		return PromptResult{}, err
	}

	runCtx, runCancel := context.WithCancel(ctx)
	runID := uuid.NewString()
	run := newActivePromptRun(runID, runCancel)
	if !sess.setActiveRun(run) {
		runCancel()
		return PromptResult{}, NewRPCError(CodeInvalidParams, "a prompt is already active for this session")
	}

	var permWG sync.WaitGroup
	defer func() {
		run.cancelAllPermissions()
		runCancel()
		permWG.Wait()
		sess.clearActiveRun(run)
	}()

	preExistingIDs := make(map[string]struct{})
	if existingMsgs, listErr := s.backend.ListSessionMessages(ctx, sess.workspaceID, params.SessionID); listErr == nil {
		for _, m := range existingMsgs {
			if m.ID != "" {
				preExistingIDs[m.ID] = struct{}{}
			}
		}
	}

	subCtx, subCancel := context.WithCancel(context.Background())
	defer subCancel()

	eventsCh, err := s.backend.SubscribeEvents(subCtx, sess.workspaceID)
	if err != nil {
		return PromptResult{}, NewRPCError(CodeInternalError, err.Error())
	}

	_, reasoningMode := sess.getConfigState()
	if err := s.backend.SendMessage(sess.workspaceID, proto.AgentMessage{
		SessionID:     params.SessionID,
		RunID:         runID,
		Prompt:        promptText,
		Attachments:   attachments,
		ReasoningMode: reasoningMode,
	}); err != nil {
		return PromptResult{}, NewRPCError(CodeInternalError, err.Error())
	}

	translator := NewStreamTranslator(params.SessionID, func(update any) {
		_ = s.sendSessionUpdate(params.SessionID, update)
	})

	var lastFinishReason message.FinishReason
	var lastFinishMessage string

	handleEvent := func(payload tea.Msg) (*notify.RunComplete, bool) {
		switch ev := payload.(type) {
		case pubsub.Event[message.Message]:
			if ev.Payload.SessionID != params.SessionID {
				return nil, false
			}
			translator.HandleMessage(ev.Payload)
			if ev.Payload.Role == message.Assistant && ev.Payload.IsFinished() {
				if fp := ev.Payload.FinishPart(); fp != nil {
					lastFinishReason = fp.Reason
					lastFinishMessage = fp.Message
				}
			}
		case message.Message:
			if ev.SessionID != params.SessionID {
				return nil, false
			}
			translator.HandleMessage(ev)
			if ev.Role == message.Assistant && ev.IsFinished() {
				if fp := ev.FinishPart(); fp != nil {
					lastFinishReason = fp.Reason
					lastFinishMessage = fp.Message
				}
			}
		case pubsub.Event[session.Session]:
			if ev.Payload.ID == params.SessionID {
				translator.HandleTodos(ev.Payload.Todos)
			}
		case session.Session:
			if ev.ID == params.SessionID {
				translator.HandleTodos(ev.Todos)
			}
		case pubsub.Event[permission.PermissionRequest]:
			if ev.Payload.SessionID == params.SessionID {
				perm := ev.Payload
				permWG.Add(1)
				go func() {
					defer permWG.Done()
					s.handlePermissionRequest(runCtx, run, sess, perm)
				}()
			}
		case permission.PermissionRequest:
			if ev.SessionID == params.SessionID {
				perm := ev
				permWG.Add(1)
				go func() {
					defer permWG.Done()
					s.handlePermissionRequest(runCtx, run, sess, perm)
				}()
			}
		case pubsub.Event[notify.RunComplete]:
			rc := ev.Payload
			if rc.SessionID == params.SessionID && (rc.RunID == "" || rc.RunID == runID) {
				return &rc, true
			}
		case notify.RunComplete:
			rc := ev
			if rc.SessionID == params.SessionID && (rc.RunID == "" || rc.RunID == runID) {
				return &rc, true
			}
		}
		return nil, false
	}

	reconcileAndFinish := func(rc *notify.RunComplete) (PromptResult, error) {
		if msgs, listErr := s.backend.ListSessionMessages(context.Background(), sess.workspaceID, params.SessionID); listErr == nil {
			for _, m := range msgs {
				if m.ID != "" {
					if _, existed := preExistingIDs[m.ID]; existed {
						continue
					}
				}
				if m.Role == message.Assistant || m.Role == message.Tool {
					translator.HandleMessage(m)
					if m.Role == message.Assistant && m.IsFinished() {
						if fp := m.FinishPart(); fp != nil {
							lastFinishReason = fp.Reason
							lastFinishMessage = fp.Message
						}
					}
				}
			}
		}
		if rc != nil {
			translator.ReconcileFinalText(rc.MessageID, rc.Text)
		}

		run.cancelAllPermissions()
		permWG.Wait()

		if run.isCancelled() || runCtx.Err() != nil || (rc != nil && rc.Cancelled) {
			return PromptResult{StopReason: StopReasonCancelled}, nil
		}
		if rc != nil && rc.Error != "" {
			return PromptResult{}, NewRPCError(CodeInternalError, rc.Error)
		}
		return mapFinishReasonToResult(lastFinishReason, lastFinishMessage)
	}

	cancelTriggered := false
	var cancelTimer *time.Timer
	var cancelTimerCh <-chan time.Time
	defer func() {
		if cancelTimer != nil {
			cancelTimer.Stop()
		}
	}()

	for {
		var doneCh <-chan struct{}
		if !cancelTriggered {
			doneCh = runCtx.Done()
		}

		select {
		case ev, ok := <-eventsCh:
			if !ok {
				return reconcileAndFinish(nil)
			}
			if rc, done := handleEvent(ev.Payload); done {
				return reconcileAndFinish(rc)
			}
		case <-doneCh:
			cancelTriggered = true
			run.markCancelled()
			run.cancelAllPermissions()
			_ = s.backend.CancelSession(sess.workspaceID, params.SessionID)
			timeout := s.cancelWaitTimeout
			if timeout <= 0 {
				timeout = defaultCancelWaitTimeout
			}
			cancelTimer = time.NewTimer(timeout)
			cancelTimerCh = cancelTimer.C
		case <-cancelTimerCh:
			return reconcileAndFinish(&notify.RunComplete{
				SessionID: params.SessionID,
				RunID:     runID,
				Cancelled: true,
			})
		}
	}
}

func mapFinishReasonToResult(reason message.FinishReason, finishMsg string) (PromptResult, error) {
	switch reason {
	case message.FinishReasonMaxTokens:
		return PromptResult{StopReason: StopReasonMaxTokens}, nil
	case message.FinishReasonContentFilter:
		return PromptResult{StopReason: StopReasonRefusal}, nil
	case message.FinishReasonCanceled:
		return PromptResult{StopReason: StopReasonCancelled}, nil
	case message.FinishReasonError:
		msg := finishMsg
		if msg == "" {
			msg = "agent run terminated with an error"
		}
		return PromptResult{}, NewRPCError(CodeInternalError, msg)
	default:
		return PromptResult{StopReason: StopReasonEndTurn}, nil
	}
}

func convertPromptBlocks(blocks []ContentBlock) (string, []proto.Attachment, error) {
	var textParts []string
	var attachments []proto.Attachment
	imageCount := 0

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case "image":
			raw, mime, err := decodeBase64Data(block.Data, block.MimeType, "image/png")
			if err != nil {
				return "", nil, NewRPCError(CodeInvalidParams, fmt.Sprintf("invalid image block: %v", err))
			}
			imageCount++
			name := block.Name
			if name == "" {
				name = fmt.Sprintf("image-%d%s", imageCount, extensionForMime(mime))
			}
			attachments = append(attachments, proto.Attachment{
				FileName: name,
				MimeType: mime,
				Content:  raw,
			})
		case "resource":
			if block.Resource == nil {
				return "", nil, NewRPCError(CodeInvalidParams, "resource block is missing resource payload")
			}
			res := block.Resource
			if res.Text != "" {
				header := res.URI
				if header == "" {
					header = "embedded-resource"
				}
				textParts = append(textParts, fmt.Sprintf("<resource uri=%q>\n%s\n</resource>", header, res.Text))
			} else if res.Blob != "" {
				raw, mime, err := decodeBase64Data(res.Blob, res.MimeType, "application/octet-stream")
				if err != nil {
					return "", nil, NewRPCError(CodeInvalidParams, fmt.Sprintf("invalid resource blob: %v", err))
				}
				name := path.Base(res.URI)
				if name == "" || name == "." || name == "/" {
					name = "resource" + extensionForMime(mime)
				}
				attachments = append(attachments, proto.Attachment{
					FilePath: res.URI,
					FileName: name,
					MimeType: mime,
					Content:  raw,
				})
			}
		case "resource_link":
			label := block.Title
			if label == "" {
				label = block.Name
			}
			var parts []string
			if label != "" {
				parts = append(parts, label)
			}
			if block.URI != "" {
				parts = append(parts, block.URI)
			}
			if block.Description != "" {
				parts = append(parts, block.Description)
			}
			if len(parts) > 0 {
				textParts = append(textParts, "[Resource: "+strings.Join(parts, " - ")+"]")
			}
		}
	}

	promptText := strings.Join(textParts, "\n\n")
	if strings.TrimSpace(promptText) == "" && len(attachments) == 0 {
		return "", nil, NewRPCError(CodeInvalidParams, "prompt must not be empty")
	}
	return promptText, attachments, nil
}

func decodeBase64Data(encoded, declaredMime, fallbackMime string) ([]byte, string, error) {
	mime := strings.TrimSpace(declaredMime)
	payload := strings.TrimSpace(encoded)
	if strings.HasPrefix(payload, "data:") {
		meta, rest, ok := strings.Cut(payload, ",")
		if ok {
			payload = rest
			if mime == "" {
				meta = strings.TrimPrefix(meta, "data:")
				if media, _, found := strings.Cut(meta, ";"); found && media != "" {
					mime = media
				}
			}
		}
	}
	if mime == "" {
		mime = fallbackMime
	}
	if payload == "" {
		return nil, "", fmt.Errorf("empty base64 data")
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return nil, "", err
		}
	}
	return raw, mime, nil
}

func extensionForMime(mime string) string {
	switch strings.ToLower(mime) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	default:
		return ".bin"
	}
}

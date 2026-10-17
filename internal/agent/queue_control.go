package agent

import (
	"context"
	"errors"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
)

var ErrQueuedPromptMissing = errors.New("queued prompt is no longer available")
var ErrQueueControlUnsupported = errors.New("agent does not support queue control")

type QueuedPrompt struct {
	ID          string `json:"id"`
	Prompt      string `json:"prompt"`
	Attachments int    `json:"attachments"`
}

type QueueController interface {
	QueueEntries(string) []QueuedPrompt
	RunQueuedNow(context.Context, string, string) (<-chan struct{}, error)
}

func (a *sessionAgent) QueueEntries(sessionID string) []QueuedPrompt {
	mu := a.sessionMu(sessionID)
	mu.Lock()
	defer mu.Unlock()
	calls, _ := a.messageQueue.Get(sessionID)
	entries := make([]QueuedPrompt, len(calls))
	for i := range calls {
		if calls[i].queueID == "" {
			calls[i].queueID = uuid.NewString()
		}
		entries[i] = QueuedPrompt{ID: calls[i].queueID, Prompt: calls[i].Prompt, Attachments: len(calls[i].Attachments)}
	}
	if len(calls) > 0 {
		a.messageQueue.Set(sessionID, calls)
	}
	return entries
}

func (a *sessionAgent) RunQueuedNow(ctx context.Context, sessionID, id string) (<-chan struct{}, error) {
	mu := a.sessionMu(sessionID)
	mu.Lock()
	calls, _ := a.messageQueue.Get(sessionID)
	index := -1
	for i := range calls {
		if calls[i].queueID == id && id != "" {
			index = i
			break
		}
	}
	if index < 0 {
		mu.Unlock()
		return nil, ErrQueuedPromptMissing
	}
	selected := calls[index]
	promoted := make([]SessionAgentCall, 0, len(calls))
	promoted = append(promoted, selected)
	promoted = append(promoted, calls[:index]...)
	promoted = append(promoted, calls[index+1:]...)
	a.messageQueue.Set(sessionID, promoted)
	var previousDone <-chan struct{}
	if active, ok := a.activeRequests.Get(sessionID); ok && active != nil {
		previousDone = active.done
		active.cancel()
	}
	mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if previousDone != nil {
			select {
			case <-previousDone:
			case <-ctx.Done():
				return
			}
		}
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			mu.Lock()
			pending, _ := a.messageQueue.Get(sessionID)
			index := -1
			for i := range pending {
				if pending[i].queueID == id {
					index = i
					break
				}
			}
			if index < 0 {
				mu.Unlock()
				return
			}
			if !a.IsSessionBusy(sessionID) {
				next := pending[index]
				rest := append([]SessionAgentCall{}, pending[:index]...)
				rest = append(rest, pending[index+1:]...)
				a.messageQueue.Set(sessionID, rest)
				next.Accepted = a.BeginAccepted(sessionID)
				mu.Unlock()
				_, _ = a.Run(ctx, next)
				return
			}
			mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return done, nil
}

func (c *coordinator) QueueEntries(sessionID string) []QueuedPrompt {
	if control, ok := c.currentAgent().(QueueController); ok {
		return control.QueueEntries(sessionID)
	}
	return []QueuedPrompt{}
}

func (c *coordinator) RunQueuedNow(ctx context.Context, sessionID, id string) (<-chan struct{}, error) {
	control, ok := c.currentAgent().(QueueController)
	if !ok {
		return nil, ErrQueueControlUnsupported
	}
	done, err := control.RunQueuedNow(ctx, sessionID, id)
	if err != nil {
		return nil, err
	}
	for _, sub := range c.cachedSubAgents() {
		sub.Cancel(sessionID)
	}
	c.cancelBackgroundAgents(sessionID)
	return done, nil
}

package mcp

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/SpherePrime/CLI/internal/pubsub"
)

func canonicalChannelWorkspace(path string) string {
	if path == "" {
		return ""
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

func SubscribeChannelEvents(ctx context.Context, workspacePath string, enabled []string) <-chan pubsub.Event[Event] {
	workspacePath = canonicalChannelWorkspace(workspacePath)
	enabled = append([]string(nil), enabled...)
	raw := broker.Subscribe(ctx)
	filtered := make(chan pubsub.Event[Event], 64)
	go func() {
		defer close(filtered)
		for event := range raw {
			if event.Payload.Type != EventChannelMessage || workspacePath == "" || event.Payload.WorkspacePath != workspacePath || !channelEnabled(enabled, event.Payload.Name) {
				continue
			}
			select {
			case filtered <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return filtered
}

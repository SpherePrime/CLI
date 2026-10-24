package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/vendordeps/modelcontextprotocol/go-sdk/mcp"
)

type Progress struct {
	At      time.Time
	Message string
	Value   float64
	Total   float64
}

var progressByToken = csync.NewMap[string, Progress]()

func recordProgress(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
	if req == nil || req.Params == nil || req.Params.ProgressToken == nil {
		return
	}
	progressByToken.Set(fmt.Sprint(req.Params.ProgressToken), Progress{
		At:      time.Now(),
		Message: req.Params.Message,
		Value:   req.Params.Progress,
		Total:   req.Params.Total,
	})
}

func LastProgress(token string) (Progress, bool) {
	return progressByToken.Get(token)
}

func ForgetProgress(token string) {
	progressByToken.Del(token)
}

func Ping(ctx context.Context, name string, wait time.Duration) error {
	sess, ok := sessions.Get(name)
	if !ok {
		return fmt.Errorf("mcp '%s' not connected", name)
	}
	return pingSession(ctx, sess, wait)
}

func HasTool(name, toolName string) bool {
	tools, ok := allTools.Get(name)
	if !ok {
		return false
	}
	for _, tool := range tools {
		if tool.Name == toolName {
			return true
		}
	}
	return false
}

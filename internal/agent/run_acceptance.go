package agent

import (
	"context"
	"sync"
)

type runAcceptanceKey struct{}

type runAcceptance struct {
	once     sync.Once
	accepted chan struct{}
}

func markRunAccepted(ctx context.Context) {
	if marker, ok := ctx.Value(runAcceptanceKey{}).(*runAcceptance); ok {
		marker.once.Do(func() { close(marker.accepted) })
	}
}

func (marker *runAcceptance) confirmed() bool {
	select {
	case <-marker.accepted:
		return true
	default:
		return false
	}
}

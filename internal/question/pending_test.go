package question

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestQuestionResolutionIsAtomicAndCorrelated(t *testing.T) {
	s := NewService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := s.Subscribe(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Ask(ctx, Request{ID: "batch", SessionID: "session", Questions: []Question{{ID: "q", Type: TypeFreeText, Text: "Name?", Description: "Your name"}}})
	}()
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("question not published")
	}
	if pending := s.Pending(); pending == nil || pending.ID != "batch" {
		t.Fatal("missing pending question")
	}
	if s.AnswerID("old-batch", nil) {
		t.Fatal("stale answer resolved current question")
	}
	var resolved atomic.Int32
	var callers sync.WaitGroup
	for range 20 {
		callers.Go(func() {
			if s.AnswerID("batch", []Answer{{QuestionID: "q", FillInText: "name"}}) {
				resolved.Add(1)
			}
		})
	}
	callers.Wait()
	<-done
	if resolved.Load() != 1 || s.Pending() != nil {
		t.Fatalf("resolution count = %d", resolved.Load())
	}
}

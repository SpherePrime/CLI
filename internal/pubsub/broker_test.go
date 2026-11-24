package pubsub

import (
	"context"
	"sync"
	"testing"
)

func TestBrokerShutdownIdempotentAndConcurrent(t *testing.T) {
	t.Parallel()
	b := NewBroker[int]()
	ch := b.Subscribe(context.Background())

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			b.Shutdown()
		})
	}
	wg.Wait()

	if _, ok := <-ch; ok {
		t.Fatal("expected subscriber channel to be closed after Shutdown")
	}
	if b.GetSubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", b.GetSubscriberCount())
	}
}

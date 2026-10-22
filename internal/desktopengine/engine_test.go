package desktopengine

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/server"
)

func testEngine(t *testing.T, handler http.HandlerFunc) *Engine {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	engine := newEngine(ctx, cancel, handler)
	t.Cleanup(engine.Close)
	return engine
}

func TestStreamingDeliversBeforeHandlerCompletes(t *testing.T) {
	proceed := make(chan struct{})
	engine := testEngine(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: first\n\n")
		select {
		case <-proceed:
		case <-r.Context().Done():
			return
		}
		io.WriteString(w, "data: second\n\n")
	})
	id, err := engine.Request(Request{Method: "GET", Path: "/v1/events"})
	if err != nil {
		t.Fatal(err)
	}
	header, err := engine.Read(id, time.Second)
	if err != nil || header == nil || header.Type != "headers" {
		t.Fatalf("headers: %v %v", header, err)
	}
	first, err := engine.Read(id, time.Second)
	if err != nil || first == nil || first.Data != "data: first\n\n" {
		t.Fatalf("first: %v %v", first, err)
	}
	close(proceed)
	second, _ := engine.Read(id, time.Second)
	if second == nil || second.Data != "data: second\n\n" {
		t.Fatalf("second: %v", second)
	}
	complete, _ := engine.Read(id, time.Second)
	if complete == nil || complete.Type != "complete" {
		t.Fatalf("complete: %v", complete)
	}
}

func TestCancelUnblocksBackpressure(t *testing.T) {
	finished := make(chan struct{})
	engine := testEngine(t, func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		for range eventQueueSize * 2 {
			if _, err := io.WriteString(w, strings.Repeat("x", maxChunkBytes)); err != nil {
				return
			}
		}
	})
	id, err := engine.Request(Request{Path: "/v1/events"})
	if err != nil {
		t.Fatal(err)
	}
	request, _ := engine.pending(id)
	deadline := time.After(time.Second)
	for len(request.events) < eventQueueSize {
		select {
		case <-deadline:
			t.Fatal("queue did not fill")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if err := engine.Cancel(id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("blocked writer did not cancel")
	}
	if err := engine.Release(id); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Read(id, 0); err == nil {
		t.Fatal("released request remained registered")
	}
}

func TestShutdownCancelsRequestsAndRejectsNewWork(t *testing.T) {
	started := make(chan struct{})
	engine := testEngine(t, func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() })
	if _, err := engine.Request(Request{Path: "/v1/events"}); err != nil {
		t.Fatal(err)
	}
	<-started
	engine.Close()
	engine.Close()
	if _, err := engine.Request(Request{Path: "/v1/events"}); err == nil {
		t.Fatal("closed engine accepted request")
	}
}

func TestRequestLimits(t *testing.T) {
	engine := testEngine(t, func(w http.ResponseWriter, r *http.Request) {})
	for _, path := range []string{"https://example.com/v1/test", "//example.com/v1/test", "/wrong"} {
		if _, err := engine.Request(Request{Path: path}); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	if _, err := engine.Request(Request{Path: "/v1/test", Body: strings.Repeat("x", MaxRequestBytes+1)}); err == nil {
		t.Fatal("accepted oversized request")
	}
}

func TestExistingRoutesInProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	srv := server.NewInProcess(ctx, nil)
	engine := newEngine(ctx, cancel, srv.Handler())
	t.Cleanup(engine.Close)
	for _, path := range []string{"/v1/health", "/v1/capabilities", "/v1/version", "/v1/workspaces"} {
		id, err := engine.Request(Request{Method: "GET", Path: path})
		if err != nil {
			t.Fatal(err)
		}
		var status int
		var body strings.Builder
		for {
			event, err := engine.Read(id, time.Second)
			if err != nil || event == nil {
				t.Fatalf("%s event: %v %v", path, event, err)
			}
			if event.Type == "headers" {
				status = event.Status
			}
			body.WriteString(event.Data)
			if event.Type == "complete" {
				break
			}
		}
		if status != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, status, body.String())
		}
		if err := engine.Release(id); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUTF8ChunkBoundaries(t *testing.T) {
	content := strings.Repeat("я", maxChunkBytes)
	engine := testEngine(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "x"+content) })
	id, err := engine.Request(Request{Path: "/v1/text"})
	if err != nil {
		t.Fatal(err)
	}
	var combined strings.Builder
	for {
		event, err := engine.Read(id, time.Second)
		if err != nil || event == nil {
			t.Fatalf("event: %v %v", event, err)
		}
		combined.WriteString(event.Data)
		if event.Type == "complete" {
			break
		}
	}
	if combined.String() != "x"+content {
		t.Fatal("UTF-8 stream changed")
	}
}

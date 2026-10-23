package desktopengine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/server"
)

type pendingRequest struct {
	cancel context.CancelFunc
	events chan Event
	done   chan struct{}
}

type Engine struct {
	ctx       context.Context
	cancel    context.CancelFunc
	handler   http.Handler
	server    *server.Server
	mu        sync.Mutex
	closed    bool
	nextID    uint64
	requests  map[uint64]*pendingRequest
	wg        sync.WaitGroup
	closeOnce sync.Once
}

func New(options Options) (*Engine, error) {
	if options.WorkingDir == "" {
		var err error
		options.WorkingDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	cfg, err := config.Init(options.WorkingDir, options.DataDir, options.Debug)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := server.NewInProcessWithOptions(ctx, cfg, options.AttachmentRoot)
	engine := newEngine(ctx, cancel, srv.Handler())
	engine.server = srv
	return engine, nil
}

func newEngine(ctx context.Context, cancel context.CancelFunc, handler http.Handler) *Engine {
	return &Engine{ctx: ctx, cancel: cancel, handler: handler, requests: make(map[uint64]*pendingRequest)}
}

func (e *Engine) Request(input Request) (uint64, error) {
	if len(input.Body) > MaxRequestBytes {
		return 0, errors.New("request body exceeds limit")
	}
	if !strings.HasPrefix(input.Path, "/v1/") || strings.HasPrefix(input.Path, "//") {
		return 0, errors.New("request path must start with /v1/")
	}
	ctx, cancel := context.WithCancel(e.ctx)
	req, err := http.NewRequestWithContext(ctx, input.Method, "http://desktop.local"+input.Path, strings.NewReader(input.Body))
	if err != nil {
		cancel()
		return 0, err
	}
	for key, value := range input.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" && input.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		cancel()
		return 0, errors.New("engine is closed")
	}
	if len(e.requests) >= maxPendingRequests {
		e.mu.Unlock()
		cancel()
		return 0, errors.New("too many pending requests")
	}
	e.nextID++
	id := e.nextID
	pending := &pendingRequest{cancel: cancel, events: make(chan Event, eventQueueSize), done: make(chan struct{})}
	e.requests[id] = pending
	e.wg.Add(1)
	e.mu.Unlock()
	go func() {
		defer e.wg.Done()
		defer close(pending.done)
		defer close(pending.events)
		defer cancel()
		writer := &responseWriter{ctx: ctx, events: pending.events, header: make(http.Header)}
		defer func() {
			if failure := recover(); failure != nil {
				writer.emit(Event{Type: "error", Error: fmt.Sprint(failure)})
			}
		}()
		e.handler.ServeHTTP(writer, req)
		if writer.status == 0 {
			writer.WriteHeader(http.StatusOK)
		}
		writer.emit(Event{Type: "complete"})
	}()
	return id, nil
}

func (e *Engine) pending(id uint64) (*pendingRequest, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	request, ok := e.requests[id]
	if !ok {
		return nil, errors.New("unknown request handle")
	}
	return request, nil
}

func (e *Engine) Read(id uint64, timeout time.Duration) (*Event, error) {
	request, err := e.pending(id)
	if err != nil {
		return nil, err
	}
	if timeout < 0 || timeout > time.Minute {
		timeout = time.Minute
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case event, ok := <-request.events:
		if !ok {
			return &Event{Type: "complete"}, nil
		}
		return &event, nil
	case <-timer.C:
		return nil, nil
	}
}

func (e *Engine) Cancel(id uint64) error {
	request, err := e.pending(id)
	if err != nil {
		return err
	}
	request.cancel()
	return nil
}

func (e *Engine) Release(id uint64) error {
	e.mu.Lock()
	request, ok := e.requests[id]
	if ok {
		delete(e.requests, id)
	}
	e.mu.Unlock()
	if !ok {
		return errors.New("unknown request handle")
	}
	request.cancel()
	return nil
}

func (e *Engine) Close() {
	e.closeOnce.Do(func() {
		e.mu.Lock()
		e.closed = true
		e.mu.Unlock()
		if e.server != nil {
			e.server.Backend().Shutdown()
		}
		e.cancel()
		e.wg.Wait()
		if e.server != nil {
			e.server.Backend().CloseInProcess()
		}
		e.mu.Lock()
		clear(e.requests)
		e.mu.Unlock()
	})
}

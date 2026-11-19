package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
)

const MaxLineBytes = 16 * 1024 * 1024

var (
	ErrTransportClosed = errors.New("acp transport closed")
	errLineTooLong     = errors.New("jsonrpc line exceeds maximum size")
)

type RequestHandler func(ctx context.Context, method string, params json.RawMessage) (any, error)

type NotificationHandler func(method string, params json.RawMessage)

type rawRPCFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type outgoingRequestFrame struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      RequestID `json:"id"`
	Method  string    `json:"method"`
	Params  any       `json:"params,omitempty"`
}

type outgoingNotificationFrame struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type outgoingResultFrame struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      RequestID `json:"id"`
	Result  any       `json:"result"`
}

type outgoingErrorFrame struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      *RequestID `json:"id"`
	Error   *RPCError  `json:"error"`
}

type rpcResponse struct {
	result json.RawMessage
	err    *RPCError
}

type readLineResult struct {
	line []byte
	err  error
}

type Transport struct {
	reader *bufio.Reader
	writer io.Writer

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan rpcResponse

	inflightMu sync.Mutex
	inflight   map[string]context.CancelFunc

	nextID atomic.Int64
	wg     sync.WaitGroup

	closed    chan struct{}
	closeOnce sync.Once
}

func NewTransport(r io.Reader, w io.Writer) *Transport {
	return &Transport{
		reader:   bufio.NewReaderSize(r, 64*1024),
		writer:   w,
		pending:  make(map[string]chan rpcResponse),
		inflight: make(map[string]context.CancelFunc),
		closed:   make(chan struct{}),
	}
}

func (t *Transport) Serve(ctx context.Context, onRequest RequestHandler, onNotification NotificationHandler) error {
	serveCtx, serveCancel := context.WithCancel(ctx)
	defer func() {
		serveCancel()
		t.cancelAllInflight()
		t.closeTransport()
		t.wg.Wait()
	}()

	lineCh := make(chan readLineResult, 1)
	go func() {
		defer close(lineCh)
		for {
			line, err := readNDJSONLine(t.reader, MaxLineBytes)
			select {
			case lineCh <- readLineResult{line: line, err: err}:
			case <-serveCtx.Done():
				return
			}
			if err != nil && !errors.Is(err, errLineTooLong) {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-lineCh:
			if !ok {
				return nil
			}
			if item.err != nil {
				if errors.Is(item.err, io.EOF) {
					return nil
				}
				if errors.Is(item.err, errLineTooLong) {
					t.writeError(nil, NewRPCError(CodeParseError, "JSON-RPC message exceeds maximum line size"))
					continue
				}
				return item.err
			}

			trimmed := bytes.TrimSpace(item.line)
			if len(trimmed) == 0 {
				continue
			}

			t.handleRawLine(serveCtx, trimmed, onRequest, onNotification)
		}
	}
}

func (t *Transport) handleRawLine(ctx context.Context, line []byte, onRequest RequestHandler, onNotification NotificationHandler) {
	var frame rawRPCFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		t.writeError(nil, NewRPCError(CodeParseError, "Invalid JSON"))
		return
	}

	if frame.JSONRPC != JSONRPCVersion {
		var respID *RequestID
		if len(frame.ID) > 0 && !bytes.Equal(bytes.TrimSpace(frame.ID), []byte("null")) {
			var parsed RequestID
			if err := json.Unmarshal(frame.ID, &parsed); err == nil {
				respID = &parsed
			}
		}
		t.writeError(respID, NewRPCError(CodeInvalidRequest, "Invalid JSON-RPC version"))
		return
	}

	hasID := len(bytes.TrimSpace(frame.ID)) > 0
	isResponse := frame.Method == "" && hasID && (len(frame.Result) > 0 || frame.Error != nil)
	if isResponse {
		var respID RequestID
		if err := json.Unmarshal(frame.ID, &respID); err != nil {
			slog.Warn("Ignoring ACP response with invalid request id", "error", err)
			return
		}
		t.deliverResponse(respID, rpcResponse{
			result: frame.Result,
			err:    frame.Error,
		})
		return
	}

	if frame.Method == "" {
		t.writeError(nil, NewRPCError(CodeInvalidRequest, "Missing method"))
		return
	}

	if !hasID {
		if frame.Method == "$/cancel_request" {
			var params CancelRequestParams
			if err := json.Unmarshal(frame.Params, &params); err == nil && params.RequestID.IsValid() {
				t.cancelInflight(params.RequestID.Key())
			}
			return
		}
		if onNotification != nil {
			onNotification(frame.Method, frame.Params)
		}
		return
	}

	trimmedID := bytes.TrimSpace(frame.ID)
	if bytes.Equal(trimmedID, []byte("null")) {
		t.writeError(nil, NewRPCError(CodeInvalidRequest, "Request id must not be null"))
		return
	}

	var reqID RequestID
	if err := json.Unmarshal(trimmedID, &reqID); err != nil {
		t.writeError(nil, NewRPCError(CodeInvalidRequest, "Invalid request id"))
		return
	}

	if onRequest == nil {
		t.writeError(&reqID, NewRPCError(CodeMethodNotFound, fmt.Sprintf("Method not found: %s", frame.Method)))
		return
	}

	if frame.Method == "initialize" {
		res, err := onRequest(ctx, frame.Method, frame.Params)
		t.writeResponse(reqID, res, err)
		return
	}

	reqCtx, cancel := context.WithCancel(ctx)
	key := reqID.Key()
	t.registerInflight(key, cancel)

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer t.unregisterInflight(key, cancel)
		res, err := onRequest(reqCtx, frame.Method, frame.Params)
		t.writeResponse(reqID, res, err)
	}()
}

func (t *Transport) Call(ctx context.Context, method string, params any, result any) error {
	reqID := IntRequestID(t.nextID.Add(1))
	key := reqID.Key()
	respCh := make(chan rpcResponse, 1)

	t.pendingMu.Lock()
	select {
	case <-t.closed:
		t.pendingMu.Unlock()
		return ErrTransportClosed
	default:
	}
	t.pending[key] = respCh
	t.pendingMu.Unlock()

	if err := t.writeFrame(outgoingRequestFrame{
		JSONRPC: JSONRPCVersion,
		ID:      reqID,
		Method:  method,
		Params:  params,
	}); err != nil {
		t.deletePending(key)
		return err
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return ErrTransportClosed
		}
		if resp.err != nil {
			return resp.err
		}
		if result != nil && len(resp.result) > 0 {
			if err := json.Unmarshal(resp.result, result); err != nil {
				return fmt.Errorf("failed to decode response for %s: %w", method, err)
			}
		}
		return nil
	case <-ctx.Done():
		t.deletePending(key)
		_ = t.Notify("$/cancel_request", CancelRequestParams{RequestID: reqID})
		return ctx.Err()
	case <-t.closed:
		t.deletePending(key)
		return ErrTransportClosed
	}
}

func (t *Transport) Notify(method string, params any) error {
	return t.writeFrame(outgoingNotificationFrame{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	})
}

func (t *Transport) writeResponse(id RequestID, result any, err error) {
	if err != nil {
		var rpcErr *RPCError
		if errors.As(err, &rpcErr) {
			t.writeError(&id, rpcErr)
			return
		}
		t.writeError(&id, NewRPCError(CodeInternalError, err.Error()))
		return
	}
	if result == nil {
		result = struct{}{}
	}
	if writeErr := t.writeFrame(outgoingResultFrame{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}); writeErr != nil {
		slog.Warn("Failed to write ACP response", "id", id.String(), "error", writeErr)
	}
}

func (t *Transport) writeError(id *RequestID, rpcErr *RPCError) {
	if writeErr := t.writeFrame(outgoingErrorFrame{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   rpcErr,
	}); writeErr != nil {
		slog.Warn("Failed to write ACP error response", "error", writeErr)
	}
}

func (t *Transport) writeFrame(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	_, err = t.writer.Write(data)
	return err
}

func (t *Transport) deliverResponse(id RequestID, resp rpcResponse) {
	key := id.Key()
	t.pendingMu.Lock()
	ch, ok := t.pending[key]
	if ok {
		delete(t.pending, key)
	}
	t.pendingMu.Unlock()
	if ok {
		ch <- resp
		close(ch)
	}
}

func (t *Transport) deletePending(key string) {
	t.pendingMu.Lock()
	delete(t.pending, key)
	t.pendingMu.Unlock()
}

func (t *Transport) registerInflight(key string, cancel context.CancelFunc) {
	t.inflightMu.Lock()
	t.inflight[key] = cancel
	t.inflightMu.Unlock()
}

func (t *Transport) unregisterInflight(key string, cancel context.CancelFunc) {
	cancel()
	t.inflightMu.Lock()
	delete(t.inflight, key)
	t.inflightMu.Unlock()
}

func (t *Transport) cancelInflight(key string) {
	t.inflightMu.Lock()
	cancel, ok := t.inflight[key]
	t.inflightMu.Unlock()
	if ok && cancel != nil {
		cancel()
	}
}

func (t *Transport) cancelAllInflight() {
	t.inflightMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(t.inflight))
	for _, cancel := range t.inflight {
		cancels = append(cancels, cancel)
	}
	t.inflightMu.Unlock()
	for _, cancel := range cancels {
		if cancel != nil {
			cancel()
		}
	}
}

func (t *Transport) closeTransport() {
	t.closeOnce.Do(func() {
		close(t.closed)
		t.pendingMu.Lock()
		for k, ch := range t.pending {
			close(ch)
			delete(t.pending, k)
		}
		t.pendingMu.Unlock()
	})
}

func readNDJSONLine(r *bufio.Reader, maxBytes int) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.ReadSlice('\n')
		if len(buf)+len(chunk) > maxBytes {
			for errors.Is(err, bufio.ErrBufferFull) {
				_, err = r.ReadSlice('\n')
			}
			return nil, errLineTooLong
		}
		buf = append(buf, chunk...)
		if err == nil {
			return buf, nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(buf) > 0 {
				return buf, nil
			}
			return nil, io.EOF
		}
		return nil, err
	}
}

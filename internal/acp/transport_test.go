package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestRequestID_JSON(t *testing.T) {
	t.Parallel()

	var intID RequestID
	require.NoError(t, json.Unmarshal([]byte("42"), &intID))
	require.True(t, intID.IsValid())
	require.Equal(t, "42", intID.String())
	rawInt, err := json.Marshal(intID)
	require.NoError(t, err)
	require.Equal(t, "42", string(rawInt))

	var strID RequestID
	require.NoError(t, json.Unmarshal([]byte(`"42"`), &strID))
	require.True(t, strID.IsValid())
	require.Equal(t, "42", strID.String())
	require.NotEqual(t, intID.Key(), strID.Key())

	var nullID RequestID
	require.Error(t, json.Unmarshal([]byte("null"), &nullID))

	var floatID RequestID
	require.Error(t, json.Unmarshal([]byte("1.5"), &floatID))

	var boolID RequestID
	require.Error(t, json.Unmarshal([]byte("true"), &boolID))
}

func TestTransport_NDJSONFramingAndLargePayload(t *testing.T) {
	t.Parallel()

	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()
	defer s2cR.Close()

	tr := NewTransport(c2sR, s2cW)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- tr.Serve(context.Background(), func(_ context.Context, method string, params json.RawMessage) (any, error) {
			require.Equal(t, "echo", method)
			var payload map[string]string
			require.NoError(t, json.Unmarshal(params, &payload))
			return map[string]string{"echoed": payload["text"]}, nil
		}, nil)
		_ = s2cW.Close()
	}()

	largeText := strings.Repeat("line-one\nline-two\n", 10000)
	require.Greater(t, len(largeText), 128*1024)

	reqBytes, err := json.Marshal(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      1,
		"method":  "echo",
		"params":  map[string]string{"text": largeText},
	})
	require.NoError(t, err)
	_, err = c2sW.Write(append(reqBytes, '\n'))
	require.NoError(t, err)

	reader := bufio.NewReaderSize(s2cR, 64*1024)
	respLine, err := readNDJSONLine(reader, MaxLineBytes)
	require.NoError(t, err)
	require.True(t, bytes.HasSuffix(respLine, []byte("\n")))
	require.Equal(t, 1, bytes.Count(respLine, []byte("\n")))

	var resp RPCMessage
	require.NoError(t, json.Unmarshal(respLine, &resp))
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.ID)
	require.Equal(t, "1", resp.ID.String())

	var result map[string]string
	require.NoError(t, json.Unmarshal(resp.Result, &result))
	require.Equal(t, largeText, result["echoed"])

	require.NoError(t, c2sW.Close())
	require.NoError(t, <-serveErr)
}

func TestTransport_InvalidJSONAndNullID(t *testing.T) {
	t.Parallel()

	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()
	defer s2cR.Close()

	tr := NewTransport(c2sR, s2cW)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- tr.Serve(context.Background(), func(_ context.Context, _ string, _ json.RawMessage) (any, error) {
			return map[string]any{}, nil
		}, nil)
		_ = s2cW.Close()
	}()

	_, err := io.WriteString(c2sW, "{not valid json\n")
	require.NoError(t, err)

	reader := bufio.NewReader(s2cR)
	line1, err := readNDJSONLine(reader, MaxLineBytes)
	require.NoError(t, err)
	var errResp1 map[string]any
	require.NoError(t, json.Unmarshal(line1, &errResp1))
	require.Nil(t, errResp1["id"])
	errObj1 := errResp1["error"].(map[string]any)
	require.Equal(t, float64(CodeParseError), errObj1["code"])

	_, err = io.WriteString(c2sW, `{"jsonrpc":"2.0","id":null,"method":"test"}`+"\n")
	require.NoError(t, err)

	line2, err := readNDJSONLine(reader, MaxLineBytes)
	require.NoError(t, err)
	var errResp2 map[string]any
	require.NoError(t, json.Unmarshal(line2, &errResp2))
	require.Nil(t, errResp2["id"])
	errObj2 := errResp2["error"].(map[string]any)
	require.Equal(t, float64(CodeInvalidRequest), errObj2["code"])

	require.NoError(t, c2sW.Close())
	require.NoError(t, <-serveErr)
}

func TestTransport_BidirectionalCallAndResponse(t *testing.T) {
	t.Parallel()

	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()
	defer s2cR.Close()

	tr := NewTransport(c2sR, s2cW)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- tr.Serve(context.Background(), func(ctx context.Context, method string, _ json.RawMessage) (any, error) {
			require.Equal(t, "trigger", method)
			var clientReply map[string]string
			if err := tr.Call(ctx, "client/ask", map[string]string{"q": "ok?"}, &clientReply); err != nil {
				return nil, err
			}
			return map[string]string{"answer": clientReply["a"]}, nil
		}, nil)
		_ = s2cW.Close()
	}()

	_, err := io.WriteString(c2sW, `{"jsonrpc":"2.0","id":"req-1","method":"trigger"}`+"\n")
	require.NoError(t, err)

	reader := bufio.NewReader(s2cR)
	outgoingLine, err := readNDJSONLine(reader, MaxLineBytes)
	require.NoError(t, err)

	var outReq RPCMessage
	require.NoError(t, json.Unmarshal(outgoingLine, &outReq))
	require.Equal(t, "client/ask", outReq.Method)
	require.NotNil(t, outReq.ID)

	replyBytes, err := json.Marshal(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      outReq.ID,
		"result":  map[string]string{"a": "yes"},
	})
	require.NoError(t, err)
	_, err = c2sW.Write(append(replyBytes, '\n'))
	require.NoError(t, err)

	finalLine, err := readNDJSONLine(reader, MaxLineBytes)
	require.NoError(t, err)

	var finalResp RPCMessage
	require.NoError(t, json.Unmarshal(finalLine, &finalResp))
	require.Nil(t, finalResp.Error)
	require.Equal(t, "req-1", finalResp.ID.String())

	var result map[string]string
	require.NoError(t, json.Unmarshal(finalResp.Result, &result))
	require.Equal(t, "yes", result["answer"])

	require.NoError(t, c2sW.Close())
	require.NoError(t, <-serveErr)
}

func TestTransport_CancelBothDirections(t *testing.T) {
	t.Parallel()

	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()
	defer s2cR.Close()

	tr := NewTransport(c2sR, s2cW)

	started := make(chan struct{})
	outgoingErrCh := make(chan error, 1)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- tr.Serve(context.Background(), func(ctx context.Context, method string, _ json.RawMessage) (any, error) {
			require.Equal(t, "slow", method)
			close(started)
			var ignored map[string]any
			err := tr.Call(ctx, "client/wait", map[string]any{}, &ignored)
			outgoingErrCh <- err
			return map[string]string{"status": "cancelled"}, nil
		}, nil)
		_ = s2cW.Close()
	}()

	_, err := io.WriteString(c2sW, `{"jsonrpc":"2.0","id":7,"method":"slow"}`+"\n")
	require.NoError(t, err)

	<-started

	reader := bufio.NewReader(s2cR)
	outLine, err := readNDJSONLine(reader, MaxLineBytes)
	require.NoError(t, err)
	var outReq RPCMessage
	require.NoError(t, json.Unmarshal(outLine, &outReq))
	require.Equal(t, "client/wait", outReq.Method)
	require.NotNil(t, outReq.ID)

	_, err = io.WriteString(c2sW, `{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":7}}`+"\n")
	require.NoError(t, err)

	var sawCancelNotify bool
	var sawFinalResp bool
	for !(sawCancelNotify && sawFinalResp) {
		line, readErr := readNDJSONLine(reader, MaxLineBytes)
		require.NoError(t, readErr)
		var msg RPCMessage
		require.NoError(t, json.Unmarshal(line, &msg))
		if msg.Method == "$/cancel_request" {
			var cp CancelRequestParams
			require.NoError(t, json.Unmarshal(msg.Params, &cp))
			require.Equal(t, outReq.ID.Key(), cp.RequestID.Key())
			sawCancelNotify = true
		} else if msg.ID != nil && msg.ID.String() == "7" {
			sawFinalResp = true
		}
	}

	select {
	case outErr := <-outgoingErrCh:
		require.ErrorIs(t, outErr, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for outgoing call cancellation")
	}

	require.NoError(t, c2sW.Close())
	require.NoError(t, <-serveErr)
}

func TestTransport_CleanEOFShutdown(t *testing.T) {
	t.Parallel()

	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()
	defer s2cR.Close()

	tr := NewTransport(c2sR, s2cW)

	started := make(chan struct{})
	canceled := make(chan struct{})

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- tr.Serve(context.Background(), func(ctx context.Context, _ string, _ json.RawMessage) (any, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			return nil, ctx.Err()
		}, nil)
		_ = s2cW.Close()
	}()

	_, err := io.WriteString(c2sW, `{"jsonrpc":"2.0","id":1,"method":"hang"}`+"\n")
	require.NoError(t, err)

	<-started
	require.NoError(t, c2sW.Close())

	go func() {
		_, _ = io.Copy(io.Discard, s2cR)
	}()

	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("expected in-flight handler context to be canceled on EOF")
	}

	require.NoError(t, <-serveErr)
}

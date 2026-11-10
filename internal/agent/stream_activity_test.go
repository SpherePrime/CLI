package agent

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func keepaliveServer(t *testing.T, gap, total time.Duration) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		end := time.Now().Add(total)
		for time.Now().Before(end) {
			if gap > 0 {
				_, _ = io.WriteString(w, ": keepalive\n\n")
				flusher.Flush()
				time.Sleep(gap)
			} else {
				time.Sleep(10 * time.Millisecond)
			}
		}
		_, _ = io.WriteString(w, "data: done\n\n")
	}))
	t.Cleanup(server.Close)
	return server
}

func streamThrough(t *testing.T, url string, timeout time.Duration) (string, error) {
	client := withStreamActivityTransport(nil)
	inner := &fakeLanguageModel{}
	var body string
	var readErr error
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		req, err := http.NewRequestWithContext(inner.streamCtx, http.MethodGet, url, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		if err != nil {
			readErr = err
			return
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		body, readErr = string(data), err
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "ok"})
	}
	stream, err := newRequestTimeoutModel(inner, timeout).Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	for range stream {
	}
	return body, readErr
}

func TestStreamActivityKeepsSilentModelAlive(t *testing.T) {
	t.Parallel()
	server := keepaliveServer(t, 20*time.Millisecond, 400*time.Millisecond)
	body, err := streamThrough(t, server.URL, 150*time.Millisecond)
	require.NoError(t, err)
	require.Contains(t, body, "data: done")
}

func TestStreamActivityStillAbortsWithoutBytes(t *testing.T) {
	t.Parallel()
	server := keepaliveServer(t, 0, 600*time.Millisecond)
	_, err := streamThrough(t, server.URL, 150*time.Millisecond)
	require.Error(t, err)
}

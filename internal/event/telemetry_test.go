package event

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoggerFoldsRepeatedMessages(t *testing.T) {
	rec := captureLogs(t)
	l := newLogger()
	now := time.Now()
	l.now = func() time.Time { return now }

	for range 5 {
		l.Warnf("sending request - %s", "Post \"https://data.example/batch/\": dial tcp: lookup data.example: no such host")
	}

	lines := rec.messages()
	if len(lines) != 1 {
		t.Fatalf("printed %d lines, want 1: %#v", len(lines), lines)
	}

	now = now.Add(foldWindow + time.Second)
	l.Warnf("sending request - %s", "Post \"https://data.example/batch/\": dial tcp: lookup data.example: no such host")

	lines = rec.messages()
	if len(lines) != 2 {
		t.Fatalf("printed %d lines after the window, want 2: %#v", len(lines), lines)
	}
	if !strings.Contains(lines[1], "4 repeats folded") {
		t.Fatalf("second line does not mention the folded repeats: %q", lines[1])
	}
}

func TestLoggerKeepsDistinctMessagesApart(t *testing.T) {
	rec := captureLogs(t)
	l := newLogger()

	l.Warnf("first complaint")
	l.Errorf("second complaint")

	lines := rec.messages()
	if len(lines) != 2 {
		t.Fatalf("printed %d lines, want 2: %#v", len(lines), lines)
	}
}

func TestLoggerKeepsTelemetryBelowErrorLevel(t *testing.T) {
	rec := captureLogs(t)
	l := newLogger()

	l.Errorf("%d messages dropped after %d attempts", 1, 4)
	l.Warnf("sending request - %s", "no such host")

	for i, level := range rec.levels() {
		if level >= slog.LevelError {
			t.Fatalf("record %d logged at %s, want below error", i, level)
		}
	}
}

func TestBreakerTripsAfterRepeatedUnreachableBatches(t *testing.T) {
	closed := make(chan struct{}, 4)
	b := newDeliveryBreaker(func() error {
		closed <- struct{}{}
		return nil
	})

	b.Failure(nil, batchFailure(&net.DNSError{Err: "no such host", Name: "data.example"}))
	if !b.enabled() {
		t.Fatal("telemetry stopped after a single failed batch")
	}
	b.Failure(nil, batchFailure(&net.OpError{Op: "dial", Net: "tcp", Err: os.ErrPermission}))
	waitDisabled(t, b)

	if !awaitClosed(closed, time.Second) {
		t.Fatal("the client was never shut down")
	}
	b.Failure(nil, batchFailure(&net.DNSError{Err: "no such host", Name: "data.example"}))
	if awaitClosed(closed, 100 * time.Millisecond) {
		t.Fatal("a tripped breaker shut the client down again")
	}
}

func TestBreakerCountsOneBatchOnce(t *testing.T) {
	b := newDeliveryBreaker(nil)
	err := batchFailure(&net.DNSError{Err: "no such host", Name: "data.example"})

	for range 10 {
		b.Failure(nil, err)
	}
	if !b.enabled() {
		t.Fatal("one batch of dropped messages stopped telemetry")
	}
	if got := b.batches; got != 1 {
		t.Fatalf("counted %d batches, want 1", got)
	}
}

func TestBreakerIgnoresFailuresItCannotLearnFrom(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"server rejected the batch", batchFailure(errors.New("502 bad gateway"))},
		{"upload timed out", batchFailure(&net.OpError{Op: "write", Net: "tcp", Err: os.ErrDeadlineExceeded})},
		{"payload could not be marshalled", errors.New("json: unsupported value")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newDeliveryBreaker(nil)
			for range 10 {
				b.Failure(nil, tt.err)
			}
			if !b.enabled() {
				t.Fatal("telemetry stopped on a failure that says nothing about reachability")
			}
		})
	}
}

func TestBreakerSuccessResetsTheStreak(t *testing.T) {
	b := newDeliveryBreaker(nil)
	b.Failure(nil, batchFailure(&net.DNSError{Err: "no such host", Name: "data.example"}))
	b.Success(nil)
	b.Failure(nil, batchFailure(&net.DNSError{Err: "no such host", Name: "data.example"}))

	if !b.enabled() {
		t.Fatal("a delivered batch should clear the unreachable streak")
	}
	if got := b.batches; got != 1 {
		t.Fatalf("counted %d batches, want 1", got)
	}
}

func TestTelemetryOpenNeedsAClientAndAFreshBreaker(t *testing.T) {
	originalClient, originalDelivery := client, delivery
	t.Cleanup(func() { client, delivery = originalClient, originalDelivery })

	client = nil
	delivery = newDeliveryBreaker(nil)
	if telemetryOpen() {
		t.Fatal("telemetry is open without a client")
	}

	tripped := newDeliveryBreaker(nil)
	tripped.trip(errors.New("no such host"))
	if tripped.enabled() {
		t.Fatal("a tripped breaker still reports telemetry as open")
	}
}

// batchFailure wraps a connection failure the way the SDK reports a dropped
// batch: the transport error inside a url.Error for the batch endpoint.
func batchFailure(err error) error {
	return &url.Error{Op: "Post", URL: "https://data.example/batch/", Err: err}
}

func waitDisabled(t *testing.T, b *deliveryBreaker) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for b.enabled() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if b.enabled() {
		t.Fatal("telemetry stayed open after two unreachable batches")
	}
}

// awaitClosed reports whether a shutdown notice arrives within the deadline.
func awaitClosed(closed <-chan struct{}, within time.Duration) bool {
	select {
	case <-closed:
		return true
	case <-time.After(within):
		return false
	}
}

// recorder collects slog records so the tests can assert on what reached the
// log instead of what was printed.
type recorder struct {
	mu      sync.Mutex
	records []recorded
}

type recorded struct {
	level slog.Level
	msg   string
}

func (r *recorder) Enabled(context.Context, slog.Level) bool { return true }

func (r *recorder) Handle(_ context.Context, rec slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, recorded{level: rec.Level, msg: rec.Message})
	return nil
}

func (r *recorder) WithAttrs([]slog.Attr) slog.Handler { return r }

func (r *recorder) WithGroup(string) slog.Handler { return r }

func (r *recorder) messages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.records))
	for _, rec := range r.records {
		out = append(out, rec.msg)
	}
	return out
}

func (r *recorder) levels() []slog.Level {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]slog.Level, 0, len(r.records))
	for _, rec := range r.records {
		out = append(out, rec.level)
	}
	return out
}

func captureLogs(t *testing.T) *recorder {
	t.Helper()
	rec := &recorder{}
	previous := slog.Default()
	slog.SetDefault(slog.New(rec))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return rec
}

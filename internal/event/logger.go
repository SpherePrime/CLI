package event

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/posthog/posthog-go"
)

const (
	// foldWindow is how long a repeated message from the telemetry client
	// stays folded into the line that was already printed. Background ingestion
	// retries the same failure on every flush, so an endpoint that cannot be
	// reached used to cost a warning per attempt for the whole session.
	foldWindow = 2 * time.Minute

	// foldTableLimit caps how many distinct messages the fold table tracks, so
	// a chatty SDK cannot grow it without bound.
	foldTableLimit = 256
)

var _ posthog.Logger = (*logger)(nil)

// logger forwards the telemetry client's background messages to slog and folds
// repeats of the same message into the first one.
type logger struct {
	mu    sync.Mutex
	folds map[string]*fold
	now   func() time.Time
}

// fold is what happened to one distinct message since it was last printed.
type fold struct {
	at         time.Time
	suppressed int
}

func newLogger() *logger {
	return &logger{
		folds: make(map[string]*fold, 8),
		now:   time.Now,
	}
}

func (l *logger) Debugf(format string, args ...any) {
	l.log(slog.LevelDebug, format, args...)
}

func (l *logger) Logf(format string, args ...any) {
	l.log(slog.LevelInfo, format, args...)
}

func (l *logger) Warnf(format string, args ...any) {
	l.log(slog.LevelWarn, format, args...)
}

// Errorf records at warning level: the only failures the client reports are its
// own delivery problems, and a laptop that cannot reach the ingestion endpoint
// must not look like Prime is failing.
func (l *logger) Errorf(format string, args ...any) {
	l.log(slog.LevelWarn, format, args...)
}

// log prints the first occurrence of a message and counts later ones until the
// fold window passes, at which point one line reports how many were hidden.
func (l *logger) log(level slog.Level, format string, args ...any) {
	if !slog.Default().Enabled(context.Background(), level) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	folded := l.record(msg, l.now())
	if folded < 0 {
		return
	}
	if folded > 0 {
		msg = fmt.Sprintf("%s (%d repeats folded since the last line)", msg, folded)
	}
	slog.Log(context.Background(), level, msg)
}

// record returns -1 when the message must stay quiet, otherwise how many
// repeats are owed a mention on the next printed line.
func (l *logger) record(msg string, now time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.folds == nil {
		l.folds = make(map[string]*fold, 8)
	}
	seen, ok := l.folds[msg]
	if !ok {
		if len(l.folds) >= foldTableLimit {
			clear(l.folds)
		}
		l.folds[msg] = &fold{at: now}
		return 0
	}
	if now.Sub(seen.at) < foldWindow {
		seen.suppressed++
		return -1
	}
	folded := seen.suppressed
	seen.at = now
	seen.suppressed = 0
	return folded
}

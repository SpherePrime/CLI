package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SpherePrime/CLI/internal/event"
	"github.com/SpherePrime/CLI/internal/home"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/term"
	"github.com/SpherePrime/CLI/vendordeps/natefinch/lumberjack.v2"
)

var (
	initOnce      sync.Once
	initialized   atomic.Bool
	activeLogFile atomic.Value
)

func DefaultDesktopLogDir() string {
	if override := strings.TrimSpace(os.Getenv("PRIME_DESKTOP_LOG_DIR")); override != "" {
		return override
	}
	return filepath.Join(home.Dir(), ".prime", "logs")
}

func DefaultDesktopLogFile() string {
	return filepath.Join(DefaultDesktopLogDir(), "desktop-engine.log")
}

func ActiveLogFile() string {
	if v, ok := activeLogFile.Load().(string); ok {
		return v
	}
	return ""
}

func Setup(logFile string, debug bool, ws ...io.Writer) {
	initOnce.Do(func() {
		if dir := filepath.Dir(logFile); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		activeLogFile.Store(logFile)

		logRotator := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     30,
			Compress:   false,
		}

		level := slog.LevelInfo
		if debug {
			level = slog.LevelDebug
		}

		opts := &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		}

		var handlers []slog.Handler
		handlers = append(handlers, slog.NewJSONHandler(logRotator, opts))

		for _, w := range ws {
			if w == nil {
				continue
			}
			if f, ok := w.(term.File); ok && term.IsTerminal(f.Fd()) {
				handlers = append(handlers, slog.NewTextHandler(w, opts))
			} else {
				handlers = append(handlers, slog.NewJSONHandler(w, opts))
			}
		}

		slog.SetDefault(slog.New(slog.NewMultiHandler(handlers...)))
		initialized.Store(true)
	})
}

func Initialized() bool {
	return initialized.Load()
}

func RecoverPanic(name string, cleanup func()) {
	if r := recover(); r != nil {
		event.Error(r, "panic", true, "name", name)

		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("prime-panic-%s-%s.log", name, timestamp)
		if current := ActiveLogFile(); current != "" {
			filename = filepath.Join(filepath.Dir(current), filename)
		}

		file, err := os.Create(filename)
		if err == nil {
			fmt.Fprintf(file, "Panic in %s: %v\n\n", name, r)
			fmt.Fprintf(file, "Time: %s\n\n", time.Now().Format(time.RFC3339))
			fmt.Fprintf(file, "Stack Trace:\n%s\n", debug.Stack())
			_ = file.Close()
		}

		if cleanup != nil {
			cleanup()
		}
	}
}

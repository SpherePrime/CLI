package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/acp"
	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/term"
	"github.com/SpherePrime/CLI/vendordeps/natefinch/lumberjack.v2"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

func init() {
	acpCmd.Flags().BoolP("yolo", "y", false, "Automatically accept all permissions (dangerous mode)")
}

type closableLogWriter struct {
	mu     sync.Mutex
	closed bool
	logger *lumberjack.Logger
}

func (w *closableLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return len(p), nil
	}
	return w.logger.Write(p)
}

func (w *closableLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	return w.logger.Close()
}

func setupACPLogging(logFile string, debug bool) func() {
	logWriter := &closableLogWriter{
		logger: &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    10,
			MaxBackups: 0,
			MaxAge:     30,
			Compress:   false,
		},
	}

	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	handlers := []slog.Handler{
		slog.NewJSONHandler(logWriter, opts),
	}
	if debug {
		if term.IsTerminal(os.Stderr.Fd()) {
			handlers = append(handlers, slog.NewTextHandler(os.Stderr, opts))
		} else {
			handlers = append(handlers, slog.NewJSONHandler(os.Stderr, opts))
		}
	}

	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewMultiHandler(handlers...)))

	return func() {
		slog.SetDefault(prev)
		_ = logWriter.Close()
	}
}

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Start ACP (Agent Client Protocol) server",
	Long:  "Start an ACP v1 (Agent Client Protocol) server over stdio NDJSON.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		dataDir, err := cmd.Flags().GetString("data-dir")
		if err != nil {
			return fmt.Errorf("failed to get data directory: %w", err)
		}
		debug, err := cmd.Flags().GetBool("debug")
		if err != nil {
			return fmt.Errorf("failed to get debug flag: %w", err)
		}
		yolo, err := cmd.Flags().GetBool("yolo")
		if err != nil {
			return fmt.Errorf("failed to get yolo flag: %w", err)
		}

		logDir := filepath.Join(config.GlobalCacheDir(), "acp")
		if mkErr := os.MkdirAll(logDir, 0o700); mkErr != nil {
			return fmt.Errorf("failed to create ACP log directory: %w", mkErr)
		}
		logFile := filepath.Join(logDir, "prime.log")
		cleanupLog := setupACPLogging(logFile, debug)
		defer cleanupLog()

		cwd, err := resolveACPCwd(cmd)
		if err != nil {
			return err
		}

		cfg, err := config.Load(cwd, dataDir, debug)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		baseCtx := cmd.Context()
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		ctx, cancel := context.WithCancel(baseCtx)
		defer cancel()

		sigch := make(chan os.Signal, 1)
		sigs := []os.Signal{os.Interrupt}
		sigs = append(sigs, addSignals(sigs)...)
		signal.Notify(sigch, sigs...)
		defer signal.Stop(sigch)

		parentCh := parentExited()
		sigDone := make(chan struct{})
		go func() {
			defer close(sigDone)
			select {
			case <-ctx.Done():
			case <-sigch:
				slog.Info("Received interrupt signal in ACP server")
				cancel()
			case <-parentCh:
				slog.Info("Parent process exited for ACP server")
				cancel()
			}
		}()
		defer func() {
			cancel()
			<-sigDone
		}()

		b := backend.New(ctx, cfg, nil)
		defer b.CloseInProcess()

		accSvc := account.NewService()
		defer accSvc.Close()

		srv := acp.NewServer(
			&acp.BackendAdapter{Backend: b},
			accSvc,
			cmd.InOrStdin(),
			cmd.OutOrStdout(),
			acp.ServerOptions{
				DefaultCwd: cwd,
				DataDir:    dataDir,
				Debug:      debug,
				YOLO:       yolo,
			},
		)

		slog.Info("Starting ACP server", "cwd", cwd, "yolo", yolo)
		if err := srv.Serve(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("ACP server exited with error", "error", err)
			return err
		}
		return nil
	},
}

func resolveACPCwd(cmd *cobra.Command) (string, error) {
	cwdFlag, _ := cmd.Flags().GetString("cwd")
	cwd := strings.TrimSpace(cwdFlag)
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		return wd, nil
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}
	info, err := os.Stat(absCwd)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("working directory does not exist or is not a directory: %s", cwd)
	}
	return absCwd, nil
}

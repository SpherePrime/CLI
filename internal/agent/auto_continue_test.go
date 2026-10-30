package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func autoContinueConfig(enabled bool, attempts int) *config.Config {
	options := &config.Options{AutoContinue: enabled}
	if attempts > 0 {
		options.AutoContinueAttempts = &attempts
	}
	return &config.Config{Options: options}
}

func TestAutoContinueEnabled(t *testing.T) {
	failure := errors.New("stream error: Too many requests")

	require.True(t, autoContinueEnabled(autoContinueConfig(true, 0), context.Background(), failure, 0))
	require.True(t, autoContinueEnabled(autoContinueConfig(true, 0), context.Background(), failure, config.DefaultAutoContinueAttempts-1))
}

func TestAutoContinueEnabledNeverForFinishedRuns(t *testing.T) {
	require.False(t, autoContinueEnabled(autoContinueConfig(true, 0), context.Background(), nil, 0))
}

func TestAutoContinueEnabledRespectsTheOption(t *testing.T) {
	failure := errors.New("boom")
	require.False(t, autoContinueEnabled(autoContinueConfig(false, 0), context.Background(), failure, 0))
	require.False(t, autoContinueEnabled(nil, context.Background(), failure, 0))
	require.False(t, autoContinueEnabled(&config.Config{}, context.Background(), failure, 0))
}

func TestAutoContinueEnabledStopsAtTheAttemptBudget(t *testing.T) {
	failure := errors.New("boom")
	require.False(t, autoContinueEnabled(autoContinueConfig(true, 0), context.Background(), failure, config.DefaultAutoContinueAttempts))
	require.True(t, autoContinueEnabled(autoContinueConfig(true, 3), context.Background(), failure, 2))
	require.False(t, autoContinueEnabled(autoContinueConfig(true, 3), context.Background(), failure, 3))
}

func TestAutoContinueEnabledNeverContinuesCancelledRuns(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	require.False(t, autoContinueEnabled(autoContinueConfig(true, 0), cancelled, errors.New("boom"), 0))
	require.False(t, autoContinueEnabled(autoContinueConfig(true, 0), context.Background(), context.Canceled, 0))
}

package customagents

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestServiceRejectsDispatchAfterStop(t *testing.T) {
	t.Parallel()
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	svc := NewService(store, nil, nil, nil)
	svc.Start(t.Context())
	svc.Stop()

	if _, err := svc.Dispatch(t.Context(), DispatchRequest{AgentID: "x"}); !errors.Is(err, ErrServiceStopped) {
		t.Fatalf("expected ErrServiceStopped from Dispatch, got %v", err)
	}
	if _, err := svc.DispatchSync(t.Context(), DispatchRequest{AgentID: "x"}); !errors.Is(err, ErrServiceStopped) {
		t.Fatalf("expected ErrServiceStopped from DispatchSync, got %v", err)
	}
	if _, err := svc.DispatchSwarm(context.Background(), SwarmRequest{Goal: "g", Workers: []SwarmWorkerSpec{{Name: "w"}}}); !errors.Is(err, ErrServiceStopped) {
		t.Fatalf("expected ErrServiceStopped from DispatchSwarm, got %v", err)
	}
}

func TestFilteredMonitorEnvStripsDesktopSecrets(t *testing.T) {
	t.Parallel()
	env := []string{
		"PATH=/usr/bin",
		"PRIME_DESKTOP_SUDO_PASSWORD=secret",
		"PRIME_DESKTOP_BROWSER_TOKEN=tok123",
		"prime_desktop_sudo_password=lower",
		"KEEP_ME=ok",
	}
	got := filteredMonitorEnv(env)
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "secret") || strings.Contains(joined, "tok123") || strings.Contains(joined, "lower") {
		t.Fatalf("desktop secrets leaked into monitor env: %v", got)
	}
	if !strings.Contains(joined, "PATH=/usr/bin") || !strings.Contains(joined, "KEEP_ME=ok") {
		t.Fatalf("non-secret env vars dropped: %v", got)
	}
}

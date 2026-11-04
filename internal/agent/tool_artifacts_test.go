package agent

import (
	"context"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestToolArtifactIsolationRestartAndPages(t *testing.T) {
	dir := t.TempDir()
	store, err := newToolArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(strings.Repeat("Ж", 20000))
	id, err := store.Put(context.Background(), "workspace/session", "call", fantasy.ToolOutcomeFailure, raw)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := store.Put(context.Background(), "workspace/session", "call", fantasy.ToolOutcomeFailure, raw)
	if err != nil || id != id2 {
		t.Fatalf("dedup: %s %s %v", id, id2, err)
	}
	store, err = newToolArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	page, next, total, err := store.Read(context.Background(), "workspace/session", id, 0, 16383)
	if err != nil || !utf8.Valid(page) || next != len(page) || total != len(raw) {
		t.Fatalf("page %d %d %v", next, total, err)
	}
	for _, scope := range []string{"workspace/other", "other/session"} {
		if _, _, _, err := store.Read(context.Background(), scope, id, 0, 10); err == nil {
			t.Fatal("foreign read")
		}
	}
	for _, id := range []string{"../escape", "/absolute", strings.Repeat("a", 64)} {
		if _, _, _, err := store.Read(context.Background(), "workspace/session", id, 0, 10); err == nil {
			t.Fatal("invalid artifact read")
		}
	}
	if _, _, _, err := store.Read(context.Background(), "workspace/session", id, 1, 10); err == nil {
		t.Fatal("invalid UTF-8 offset accepted")
	}
	if err := store.DeleteSession(context.Background(), "workspace/session"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := store.Read(context.Background(), "workspace/session", id, 0, 10); err == nil {
		t.Fatal("deleted artifact survived")
	}
}

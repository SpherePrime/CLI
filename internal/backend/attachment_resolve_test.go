package backend

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func newAttachmentBackend(t *testing.T, root string) (*Backend, *Workspace) {
	t.Helper()
	b, _ := newTestBackend(t)
	b.AttachmentRoot = root
	ws := insertAgentWorkspace(t, b, newBlockingCoordinator())
	return b, ws
}

func TestResolveAttachments_ReadsTrustedReference(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b, _ := newAttachmentBackend(t, root)
	path := filepath.Join(root, "note.txt")
	require.NoError(t, os.WriteFile(path, []byte("disk payload"), 0o600))

	resolved, err := b.resolveAttachments([]proto.Attachment{{FilePath: path, FileName: "note.txt", MimeType: "text/plain"}})
	require.NoError(t, err)
	require.Equal(t, []byte("disk payload"), resolved[0].Content)
	require.Equal(t, path, resolved[0].FilePath)
}

func TestResolveAttachments_RejectsOutsideRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b, _ := newAttachmentBackend(t, root)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))

	_, err := b.resolveAttachments([]proto.Attachment{{FilePath: outside}})
	require.Error(t, err)
}

func TestResolveAttachments_RejectsWithoutRoot(t *testing.T) {
	t.Parallel()
	b, _ := newAttachmentBackend(t, "")
	path := filepath.Join(t.TempDir(), "note.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	_, err := b.resolveAttachments([]proto.Attachment{{FilePath: path}})
	require.Error(t, err)
}

func TestResolveAttachments_RejectsPathEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b, _ := newAttachmentBackend(t, root)
	relative := filepath.Join(root, "..", "secret.txt")
	require.NoError(t, os.WriteFile(relative, []byte("secret"), 0o600))
	_, err := b.resolveAttachments([]proto.Attachment{{FilePath: relative}})
	require.Error(t, err)

	// Windows creates file symlinks only with elevated privileges, so
	// the symlink vector is exercised only where it is available.
	outside := filepath.Join(t.TempDir(), "target.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	link := filepath.Join(root, "escape.txt")
	if os.Symlink(outside, link) == nil {
		_, err := b.resolveAttachments([]proto.Attachment{{FilePath: link}})
		require.Error(t, err)
	}
}

func TestResolveAttachments_RejectsOversize(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b, _ := newAttachmentBackend(t, root)
	oversize := filepath.Join(root, "big.bin")
	require.NoError(t, os.WriteFile(oversize, make([]byte, (20<<20)+1), 0o600))
	_, err := b.resolveAttachments([]proto.Attachment{{FilePath: oversize}})
	require.Error(t, err)
}

func TestResolveAttachments_CoordinatorSeesResolvedBytes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b, ws := newAttachmentBackend(t, root)
	path := filepath.Join(root, "note.txt")
	require.NoError(t, os.WriteFile(path, []byte("disk payload"), 0o600))

	coord := newBlockingCoordinator()
	ws.App.AgentCoordinator = coord

	msg := proto.AgentMessage{
		SessionID: "S1",
		Prompt:    "hi",
		Attachments: []proto.Attachment{
			{FilePath: path, FileName: "note.txt", MimeType: "text/plain"},
		},
	}
	require.NoError(t, b.SendMessage(ws.ID, msg))
	select {
	case <-coord.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("run was not dispatched")
	}
	close(coord.release)
}

func TestResolveAttachments_PassThroughContent(t *testing.T) {
	t.Parallel()
	b, _ := newAttachmentBackend(t, t.TempDir())
	attachments := []proto.Attachment{{FileName: "a.txt", MimeType: "text/plain", Content: []byte("inline")}}
	resolved, err := b.resolveAttachments(attachments)
	require.NoError(t, err)
	require.Equal(t, []byte("inline"), resolved[0].Content)
}

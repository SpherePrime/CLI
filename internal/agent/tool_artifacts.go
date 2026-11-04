package agent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

type toolArtifactStore interface {
	Put(context.Context, string, string, fantasy.ToolOutcome, []byte) (string, error)
	Read(context.Context, string, string, int, int) ([]byte, int, int, error)
	DeleteSession(context.Context, string) error
}

type diskToolArtifactStore struct {
	path string
	mu   sync.Mutex
}
type toolArtifactManifest struct {
	CallID  string              `json:"call_id"`
	Outcome fantasy.ToolOutcome `json:"outcome"`
	SHA256  string              `json:"sha256"`
	Bytes   int                 `json:"bytes"`
}

func newToolArtifactStore(dataDir string) (toolArtifactStore, error) {
	if dataDir == "" {
		return nil, errors.New("tool result storage directory is unavailable")
	}
	path := filepath.Join(dataDir, "tool-results")
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("unsafe tool result storage directory")
	}
	return &diskToolArtifactStore{path: path}, nil
}

func artifactScope(scope string) string {
	sum := sha256.Sum256([]byte(scope))
	return hex.EncodeToString(sum[:])
}
func artifactHash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func validArtifactID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 24 && id == strings.ToLower(id)
}

func (s *diskToolArtifactStore) openScope(scope string, create bool) (*os.Root, error) {
	if scope == "" {
		return nil, errors.New("session scope is required")
	}
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	name := artifactScope(scope)
	if create {
		if err := root.MkdirAll(name, 0700); err != nil {
			return nil, err
		}
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("unsafe artifact session directory")
	}
	return root.OpenRoot(name)
}

func (s *diskToolArtifactStore) Put(ctx context.Context, scope, callID string, outcome fantasy.ToolOutcome, raw []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	root, err := s.openScope(scope, true)
	if err != nil {
		return "", err
	}
	defer root.Close()
	checksum := artifactHash(raw)
	entries, err := root.Open(".")
	if err != nil {
		return "", err
	}
	names, err := entries.Readdirnames(-1)
	entries.Close()
	if err != nil {
		return "", err
	}
	for _, name := range names {
		if !validArtifactID(name) {
			continue
		}
		manifest, data, err := readArtifact(root, name)
		if err == nil && manifest.CallID == callID && manifest.Outcome == outcome && manifest.SHA256 == checksum && string(data) == string(raw) {
			return name, nil
		}
	}
	token := make([]byte, 24)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	id := hex.EncodeToString(token)
	temp := ".tmp-" + id
	if err := root.Mkdir(temp, 0700); err != nil {
		return "", err
	}
	defer root.RemoveAll(temp)
	manifest, err := json.Marshal(toolArtifactManifest{CallID: callID, Outcome: outcome, SHA256: checksum, Bytes: len(raw)})
	if err != nil {
		return "", err
	}
	for name, data := range map[string][]byte{"data": raw, "manifest.json": manifest} {
		file, err := root.OpenFile(filepath.Join(temp, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return "", err
		}
		_, writeErr := file.Write(data)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil {
			return "", writeErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := root.Rename(temp, id); err != nil {
		return "", err
	}
	return id, nil
}

func readArtifact(root *os.Root, id string) (toolArtifactManifest, []byte, error) {
	var manifest toolArtifactManifest
	for _, name := range []string{id, filepath.Join(id, "manifest.json"), filepath.Join(id, "data")} {
		info, err := root.Lstat(name)
		if err != nil {
			return manifest, nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return manifest, nil, errors.New("artifact symlink refused")
		}
	}
	encoded, err := root.ReadFile(filepath.Join(id, "manifest.json"))
	if err != nil {
		return manifest, nil, err
	}
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		return manifest, nil, err
	}
	raw, err := root.ReadFile(filepath.Join(id, "data"))
	if err != nil {
		return manifest, nil, err
	}
	if len(raw) != manifest.Bytes || artifactHash(raw) != manifest.SHA256 {
		return manifest, nil, errors.New("artifact checksum mismatch")
	}
	return manifest, raw, nil
}

func (s *diskToolArtifactStore) Read(ctx context.Context, scope, id string, offset, limit int) ([]byte, int, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, err
	}
	if !validArtifactID(id) {
		return nil, 0, 0, errors.New("invalid artifact ID")
	}
	if limit == 0 {
		limit = 16384
	}
	if limit < 1 || limit > 16384 || offset < 0 {
		return nil, 0, 0, errors.New("invalid result page range")
	}
	root, err := s.openScope(scope, false)
	if err != nil {
		return nil, 0, 0, err
	}
	defer root.Close()
	_, raw, err := readArtifact(root, id)
	if err != nil {
		return nil, 0, 0, err
	}
	if offset > len(raw) || (offset < len(raw) && !utf8.RuneStart(raw[offset])) {
		return nil, 0, 0, errors.New("offset is not a UTF-8 boundary")
	}
	end := min(offset+limit, len(raw))
	for end > offset && end < len(raw) && !utf8.RuneStart(raw[end]) {
		end--
	}
	if end == offset && offset < len(raw) {
		return nil, 0, 0, fmt.Errorf("limit is too small for the next UTF-8 character")
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, err
	}
	return append([]byte(nil), raw[offset:end]...), end, len(raw), nil
}

func (s *diskToolArtifactStore) DeleteSession(ctx context.Context, scope string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(artifactScope(scope))
}

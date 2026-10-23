package server

import (
	"context"

	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/config"
)

func NewInProcess(ctx context.Context, cfg *config.ConfigStore) *Server {
	return NewInProcessWithOptions(ctx, cfg, "")
}

// NewInProcessWithOptions builds the in-process server with a trusted
// attachment root. Only files inside that directory resolve through
// disk-backed attachment references; an empty root disables references.
func NewInProcessWithOptions(ctx context.Context, cfg *config.ConfigStore, attachmentRoot string) *Server {
	s := &Server{backend: backend.New(ctx, cfg, nil)}
	s.backend.AttachmentRoot = attachmentRoot
	s.installHandler()
	return s
}

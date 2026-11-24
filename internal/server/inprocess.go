package server

import (
	"context"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/config"
)

func NewInProcess(ctx context.Context, cfg *config.ConfigStore) *Server {
	return NewInProcessWithOptions(ctx, cfg, "")
}

func NewInProcessWithOptions(ctx context.Context, cfg *config.ConfigStore, attachmentRoot string) *Server {
	s := &Server{
		backend: backend.New(ctx, cfg, nil),
		account: account.NewService(),
	}
	s.initCustomAgents(ctx, cfg)
	s.backend.AttachmentRoot = attachmentRoot
	s.installHandler()
	return s
}

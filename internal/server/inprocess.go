package server

import (
	"context"

	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/config"
)

func NewInProcess(ctx context.Context, cfg *config.ConfigStore) *Server {
	s := &Server{backend: backend.New(ctx, cfg, nil)}
	s.installHandler()
	return s
}

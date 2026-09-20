//go:build windows
// +build windows

package client

import (
	"context"
	"net"

	"github.com/dwertyfa288/CLI/vendordeps/Microsoft/go-winio"
)

func dialPipeContext(ctx context.Context, address string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, address)
}

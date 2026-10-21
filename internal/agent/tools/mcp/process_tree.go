package mcp

import (
	"context"
	"log/slog"

	"github.com/SpherePrime/CLI/internal/proctree"
	"github.com/SpherePrime/CLI/vendordeps/modelcontextprotocol/go-sdk/mcp"
)

type processTreeTransport struct {
	inner *mcp.CommandTransport
}

func withProcessTree(transport mcp.Transport) mcp.Transport {
	command, ok := transport.(*mcp.CommandTransport)
	if !ok {
		return transport
	}
	return &processTreeTransport{inner: command}
}

func (t *processTreeTransport) unwrapTransport() mcp.Transport { return t.inner }

func (t *processTreeTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	cmd := t.inner.Command
	proctree.Prepare(cmd)
	conn, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	group, err := proctree.Attach(cmd)
	if err != nil {
		slog.Warn("MCP process tree is not tracked", "command", cmd.Path, "error", err)
	}
	stop := context.AfterFunc(ctx, group.Kill)
	return &processTreeConnection{Connection: conn, group: group, stop: stop}, nil
}

type processTreeConnection struct {
	mcp.Connection
	group *proctree.Group
	stop  func() bool
}

func (c *processTreeConnection) Close() error {
	err := c.Connection.Close()
	c.stop()
	c.group.Kill()
	return err
}

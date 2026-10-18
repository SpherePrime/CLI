package server

import (
	"context"
	"github.com/SpherePrime/CLI/vendordeps/Microsoft/go-winio"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"testing"
	"time"
)

func TestPrivatePipeAcceptsCurrentUser(t *testing.T) {
	address := `\\.\pipe\prime-desktop-test-` + uuid.NewString()
	listener, _, err := listen("npipe", address)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			connection.Close()
		}
		accepted <- err
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	connection, err := winio.DialPipeContext(ctx, address)
	if err != nil {
		t.Fatal(err)
	}
	connection.Close()
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
}

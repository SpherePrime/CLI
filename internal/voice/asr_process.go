package voice

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/voice/asrpb"
	"github.com/SpherePrime/CLI/vendordeps/grpc"
	"github.com/SpherePrime/CLI/vendordeps/grpc/credentials/insecure"
	"github.com/SpherePrime/CLI/vendordeps/grpc/metadata"
)

type asrHandshake struct {
	Addr    string   `json:"addr"`
	Token   string   `json:"token"`
	Langs   []string `json:"langs"`
	ChunkMs uint32   `json:"chunk_ms"`
}

type asrClientPool struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	conn    *grpc.ClientConn
	client  asrpb.AsrServiceClient
	token   string
	dataDir string
	timer   *time.Timer
}

var sharedAsrPool = &asrClientPool{}

const asrIdleTimeout = 2 * time.Minute

func (p *asrClientPool) acquire(ctx context.Context, dataDir string) (asrpb.AsrServiceClient, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client != nil && p.cmd != nil && p.cmd.ProcessState == nil && p.dataDir == dataDir {
		p.resetIdleLocked()
		return p.client, p.token, nil
	}

	p.closeLocked()

	assets, err := EnsureAsrAssets(ctx, dataDir, nil)
	if err != nil {
		return nil, "", err
	}

	cmd := exec.Command(assets.BinaryPath, "--asr", assets.AsrModelPath, "--vad", assets.VadModelPath)
	cmd.SysProcAttr = lowPrioritySysAttr()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, "", fmt.Errorf("prime-asr stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, "", fmt.Errorf("prime-asr stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, "", fmt.Errorf("start prime-asr: %w", err)
	}

	handshakeCh := make(chan asrHandshake, 1)
	errCh := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			errCh <- fmt.Errorf("read prime-asr handshake: %w", readErr)
			return
		}
		var hs asrHandshake
		if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(line)), &hs); jsonErr != nil {
			errCh <- fmt.Errorf("parse prime-asr handshake: %w", jsonErr)
			return
		}
		if hs.Addr == "" || hs.Token == "" {
			errCh <- errors.New("incomplete prime-asr handshake")
			return
		}
		handshakeCh <- hs
	}()

	var hs asrHandshake
	select {
	case hs = <-handshakeCh:
	case readErr := <-errCh:
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, "", readErr
	case <-ctx.Done():
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, "", ctx.Err()
	case <-time.After(30 * time.Second):
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, "", errors.New("timed out waiting for prime-asr handshake")
	}

	conn, err := grpc.NewClient(
		hs.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(asrpb.Codec())),
	)
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, "", fmt.Errorf("dial prime-asr gRPC: %w", err)
	}

	client := asrpb.NewAsrServiceClient(conn)
	checkCtx, cancel := context.WithTimeout(withAsrToken(ctx, hs.Token), 5*time.Second)
	defer cancel()
	if _, err := client.Status(checkCtx, &asrpb.StatusRequest{}); err != nil {
		_ = conn.Close()
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, "", fmt.Errorf("prime-asr Status RPC failed: %w", err)
	}

	p.cmd = cmd
	p.stdin = stdin
	p.conn = conn
	p.client = client
	p.token = hs.Token
	p.dataDir = dataDir
	p.resetIdleLocked()
	return p.client, p.token, nil
}

func (p *asrClientPool) resetIdleLocked() {
	if p.timer != nil {
		p.timer.Stop()
	}
	p.timer = time.AfterFunc(asrIdleTimeout, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.closeLocked()
	})
}

func (p *asrClientPool) invalidate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *asrClientPool) closeLocked() {
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	if p.client != nil && p.token != "" {
		ctx, cancel := context.WithTimeout(withAsrToken(context.Background(), p.token), 500*time.Millisecond)
		_, _ = p.client.Shutdown(ctx, &asrpb.ShutdownRequest{})
		cancel()
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
	if p.stdin != nil {
		_ = p.stdin.Close()
		p.stdin = nil
	}
	if p.cmd != nil {
		done := make(chan struct{})
		cmd := p.cmd
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-done
		}
		p.cmd = nil
	}
	p.client = nil
	p.token = ""
	p.dataDir = ""
}

func CloseAsr() {
	sharedAsrPool.invalidate()
}

func withAsrToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, asrpb.TokenHeader, token)
}

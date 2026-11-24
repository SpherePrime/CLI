package customagents

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	maxMonitorBytes   = 512 * 1024
	monitorTimeout    = 20 * time.Second
	maxMonitorDiffLen = 4000
)

func Probe(ctx context.Context, agent CustomAgent) ProbeResult {
	var (
		snapshot string
		err      error
	)

	switch agent.MonitorKind {
	case MonitorFile:
		snapshot, err = probeFile(agent)
	case MonitorCommand:
		snapshot, err = probeCommand(ctx, agent)
	case MonitorHTTP:
		snapshot, err = probeHTTP(ctx, agent)
	default:
		return ProbeResult{Error: fmt.Sprintf("неизвестный тип мониторинга: %q", agent.MonitorKind)}
	}

	if err != nil {
		return ProbeResult{Error: err.Error()}
	}

	hash := HashBytes([]byte(snapshot))
	changed := agent.MonitorLastHash == "" || hash != agent.MonitorLastHash
	diff := ""
	if changed && agent.MonitorLastHash != "" {
		diff = UnifiedLineDiff(agent.MonitorLastSnapshot, snapshot, maxMonitorDiffLen)
	}

	return ProbeResult{
		Changed:  changed,
		Hash:     hash,
		Snapshot: snapshot,
		Diff:     diff,
	}
}

func probeFile(agent CustomAgent) (string, error) {
	target := strings.TrimSpace(agent.MonitorTarget)
	if target == "" {
		return "", fmt.Errorf("не указан файл для мониторинга")
	}
	if !filepath.IsAbs(target) && agent.WorkspacePath != "" {
		target = filepath.Join(agent.WorkspacePath, target)
	}

	f, err := os.Open(target)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения файла %s: %w", target, err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxMonitorBytes))
	if err != nil {
		return "", fmt.Errorf("ошибка чтения файла %s: %w", target, err)
	}
	return string(data), nil
}

func probeCommand(ctx context.Context, agent CustomAgent) (string, error) {
	target := strings.TrimSpace(agent.MonitorTarget)
	if target == "" {
		return "", fmt.Errorf("не указана команда для мониторинга")
	}

	cmdCtx, cancel := context.WithTimeout(ctx, monitorTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "cmd", "/C", target)
	} else {
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", target)
	}
	if agent.WorkspacePath != "" {
		cmd.Dir = agent.WorkspacePath
	}
	cmd.Env = filteredMonitorEnv(os.Environ())

	var buf limitedBuffer
	buf.limit = maxMonitorBytes
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(buf.String())
		if out != "" {
			return "", fmt.Errorf("команда завершилась с ошибкой (%w): %s", err, out)
		}
		return "", fmt.Errorf("команда завершилась с ошибкой: %w", err)
	}
	return buf.String(), nil
}

func filteredMonitorEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(name, "PRIME_DESKTOP_SUDO_PASSWORD") ||
			strings.EqualFold(name, "PRIME_DESKTOP_BROWSER_TOKEN") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func probeHTTP(ctx context.Context, agent CustomAgent) (string, error) {
	rawURL := strings.TrimSpace(agent.MonitorTarget)
	if rawURL == "" {
		return "", fmt.Errorf("не указан URL для мониторинга")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("некорректный URL %q: %w", rawURL, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("неподдерживаемая схема URL %q", parsed.Scheme)
	}
	if isBlockedMonitorHost(parsed.Hostname()) {
		return "", fmt.Errorf("запрещённый адрес мониторинга: %s", parsed.Hostname())
	}

	reqCtx, cancel := context.WithTimeout(ctx, monitorTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("ошибка создания HTTP-запроса: %w", err)
	}

	client := &http.Client{
		Timeout: monitorTimeout,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("слишком много редиректов")
			}
			s := strings.ToLower(r.URL.Scheme)
			if s != "http" && s != "https" {
				return fmt.Errorf("запрещённый редирект на схему %q", r.URL.Scheme)
			}
			if isBlockedMonitorHost(r.URL.Hostname()) {
				return fmt.Errorf("запрещённый редирект на %s", r.URL.Hostname())
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка HTTP-запроса: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMonitorBytes))
	if err != nil {
		return "", fmt.Errorf("ошибка чтения HTTP-ответа: %w", err)
	}

	return fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body)), nil
}

func isBlockedMonitorHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return true
	}
	if h == "metadata.google.internal" || h == "metadata.goog" {
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	if ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}

type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.limit > 0 && l.buf.Len() < l.limit {
		remain := l.limit - l.buf.Len()
		if len(p) > remain {
			l.buf.Write(p[:remain])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}

func (l *limitedBuffer) String() string {
	return l.buf.String()
}

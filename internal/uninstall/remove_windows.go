//go:build windows

package uninstall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// pathScriptTimeout bounds the PATH cleanup. The script only rewrites a
// registry-backed environment variable, so it is either instant or stuck; an
// unbounded wait would leave `prime uninstall` hanging forever with no way to
// cancel it.
const pathScriptTimeout = 30 * time.Second

const pathScript = `$d = $env:PRIME_UNINSTALL_DIR
$cur = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not $cur) { exit 0 }
$kept = ($cur -split ';' | Where-Object { $_ -and ($_ -ne $d) }) -join ';'
if ($kept -ne $cur) { [Environment]::SetEnvironmentVariable('Path', $kept, 'User') }`

func removeFromUserPath(exe string) error {
	ctx, cancel := context.WithTimeout(context.Background(), pathScriptTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", pathScript)
	cmd.Env = append(os.Environ(), "PRIME_UNINSTALL_DIR="+filepath.Dir(exe))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("editing PATH timed out after %s", pathScriptTimeout)
		}
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

const removeScript = `Start-Sleep -Seconds 4
Remove-Item -LiteralPath $env:PRIME_UNINSTALL_EXE -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $env:PRIME_UNINSTALL_DIR -Force -ErrorAction SilentlyContinue`

func removeBinary(exe string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", removeScript)
	cmd.Env = append(os.Environ(),
		"PRIME_UNINSTALL_EXE="+exe,
		"PRIME_UNINSTALL_DIR="+filepath.Dir(exe),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("schedule binary removal: %w", err)
	}
	return fmt.Sprintf("the binary will be deleted in a few seconds: %s", exe), nil
}

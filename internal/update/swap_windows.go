//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const replaceScript = `$ErrorActionPreference = "SilentlyContinue"
$exe = $env:PRIME_UPDATE_EXE
$new = $env:PRIME_UPDATE_NEW
$old = "$exe.old"
for ($i = 0; $i -lt 7200; $i++) {
  if (Test-Path -LiteralPath "\\?\$old") { Remove-Item -LiteralPath "\\?\$old" -Force }
  try {
    Move-Item -LiteralPath "\\?\$exe" -Destination "\\?\$old" -Force -ErrorAction Stop
    Move-Item -LiteralPath "\\?\$new" -Destination "\\?\$exe" -Force -ErrorAction Stop
    Remove-Item -LiteralPath "\\?\$old" -Force
    break
  } catch {
    Start-Sleep -Milliseconds 500
  }
}`

func swapBinary(exe string, data []byte) error {
	tmp := exe + ".update-new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := scheduleReplace(exe, tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("schedule binary replacement: %w", err)
	}
	return nil
}

func scheduleReplace(exe, newBin string) error {
	// The running UI and server both keep prime.exe locked; a detached
	// helper retries the swap every 500ms until every instance exits.
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", replaceScript)
	cmd.Env = append(os.Environ(),
		"PRIME_UPDATE_EXE="+exe,
		"PRIME_UPDATE_NEW="+newBin,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	}
	return cmd.Start()
}

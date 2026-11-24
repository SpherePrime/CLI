//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// The running UI and server both keep prime.exe locked, so replacing it
// requires every Prime instance to exit first.
//
// Strategy:
//  1. Write the new binary to <exe>.update-new.
//  2. If no other Prime process is running, perform the swap immediately.
//  3. Otherwise, start a detached PowerShell helper that retries the swap
//     every 500ms until all instances exit.
func swapBinary(exe string, data []byte) error {
	tmp := exe + ".update-new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}

	// Clean up any stale .old file from a previous interrupted swap.
	old := exe + ".old"
	if fi, err := os.Stat(old); err == nil && !fi.IsDir() {
		_ = os.Remove(old)
	}

	if hasRunningInstances(exe) {
		if err := scheduleReplace(exe, tmp); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("schedule binary replacement: %w", err)
		}
		return nil
	}

	if err := performSwap(exe, tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

// newPowershellCmd assembles a powershell.exe invocation for running a
// script as a child of prime. It never passes the -WindowStyle switch:
// PowerShell applies it via ShowWindow(GetConsoleWindow(), SW_HIDE), and a
// console child inherits the caller's console - so -WindowStyle Hidden
// literally hides the user's own terminal window, which is what used to
// collapse the terminal after "prime update". Instead the child is created
// with CREATE_NO_WINDOW, so it has no console window to style at all.
func newPowershellCmd(script string) *exec.Cmd {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func hasRunningInstances(selfExe string) bool {
	const script = `(Get-Process -Name "prime" -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $env:PRIME_SELF_EXE }).Count`
	cmd := newPowershellCmd(script)
	cmd.Env = append(os.Environ(), "PRIME_SELF_EXE="+selfExe)
	out, err := cmd.Output()
	if err != nil {
		return true
	}
	count := 0
	for _, c := range string(out) {
		if c >= '0' && c <= '9' {
			count = count*10 + int(c-'0')
		}
	}
	return count > 1
}

// performSwap atomically replaces the running executable with the new binary.
func performSwap(exe, newBin string) error {
	old := exe + ".old"
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("move old binary: %w", err)
	}
	if err := os.Rename(newBin, exe); err != nil {
		_ = os.Rename(old, exe)
		return fmt.Errorf("move new binary: %w", err)
	}
	_ = os.Remove(old)
	return nil
}

func scheduleReplace(exe, newBin string) error {
	const replaceScript = `$ErrorActionPreference = "SilentlyContinue"
$exe = $env:PRIME_UPDATE_EXE
$new = $env:PRIME_UPDATE_NEW
$old = "$exe.old"
for ($i = 0; $i -lt 7200; $i++) {
  if ((Test-Path -LiteralPath "\\?\$exe") -and (Test-Path -LiteralPath "\\?\$old")) {
    Remove-Item -LiteralPath "\\?\$old" -Force
  }
  $movedOld = $false
  try {
    Move-Item -LiteralPath "\\?\$exe" -Destination "\\?\$old" -Force -ErrorAction Stop
    $movedOld = $true
    Move-Item -LiteralPath "\\?\$new" -Destination "\\?\$exe" -Force -ErrorAction Stop
    if (Test-Path -LiteralPath "\\?\$exe") {
      Remove-Item -LiteralPath "\\?\$old" -Force
    }
    break
  } catch {
    if ($movedOld -and -not (Test-Path -LiteralPath "\\?\$exe") -and (Test-Path -LiteralPath "\\?\$old")) {
      Move-Item -LiteralPath "\\?\$old" -Destination "\\?\$exe" -Force
    }
    Start-Sleep -Milliseconds 500
  }
}`

	cmd := newPowershellCmd(replaceScript)
	cmd.Env = append(os.Environ(),
		"PRIME_UPDATE_EXE="+exe,
		"PRIME_UPDATE_NEW="+newBin,
	)
	cmd.SysProcAttr.CreationFlags |= 0x00000008 | 0x00000200
	return cmd.Start()
}

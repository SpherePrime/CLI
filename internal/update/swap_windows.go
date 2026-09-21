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

// hasRunningInstances reports whether at least one Prime process other than
// the current one is running.
func hasRunningInstances(selfExe string) bool {
	script := fmt.Sprintf(
		`(Get-Process -Name "prime" -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq '%s' }).Count`,
		selfExe,
	)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script).Output()
	if err != nil {
		// If we can't check, assume instances exist to be safe.
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
	// The running UI and server both keep prime.exe locked; a detached
	// helper retries the swap every 500ms until every instance exits.
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

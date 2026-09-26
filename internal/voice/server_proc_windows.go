//go:build windows

package voice

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// detachCommand gives the child its own process group with no console, so
// the whisper server and its watchdog outlive Prime's terminal and never
// flash a window.
func detachCommand(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= 0x00000008 | 0x00000200 // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
}

// consoleCommand opens the child in its own visible terminal window, which
// is what bringing back a killed Prime needs.
func consoleCommand(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x00000010 // CREATE_NEW_CONSOLE
}

// defaultSpawnDetached starts spec and hands the process back to Windows.
func defaultSpawnDetached(spec serverSpec) (int, error) {
	cmd := exec.Command(spec.Exe, spec.Args...)
	cmd.Env = mergeEnv(spec.Env)
	cmd.Stdin = nil
	if spec.Log != nil {
		cmd.Stdout = spec.Log
		cmd.Stderr = spec.Log
	}
	if spec.NewConsole {
		consoleCommand(cmd)
	} else {
		detachCommand(cmd)
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

// defaultKillProcess terminates a process Prime started.
func defaultKillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// serverKillGrace is how long a stopped whisper server may take to disappear;
// Windows kills are immediate, so it only guards log flushing.
const serverKillGrace = 500 * time.Millisecond

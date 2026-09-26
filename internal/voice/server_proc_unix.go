//go:build !windows

package voice

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// detachCommand moves the child into its own session so the whisper server
// and its watchdog survive Prime's terminal.
func detachCommand(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}

// defaultSpawnDetached starts spec and hands the process back to init. A
// visible-terminal request is refused here: unix has no safe way to conjure
// one from a detached process, and Prime only restarts visibly on Windows.
func defaultSpawnDetached(spec serverSpec) (int, error) {
	if spec.NewConsole {
		return 0, errors.New("no visible-terminal launcher on this platform")
	}
	cmd := exec.Command(spec.Exe, spec.Args...)
	cmd.Env = mergeEnv(spec.Env)
	cmd.Stdin = nil
	if spec.Log != nil {
		cmd.Stdout = spec.Log
		cmd.Stderr = spec.Log
	}
	detachCommand(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

// defaultKillProcess terminates a process Prime started. The child handle was
// released, so a released process can still become a zombie while Prime runs;
// reaping it here keeps the process table clean.
func defaultKillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	go reapDetachedChild(pid)
	return nil
}

// reapDetachedChild waits out the released child's exit. ECHILD means it is
// already gone, which is the normal end of this loop.
func reapDetachedChild(pid int) {
	for {
		if _, err := syscall.Wait4(pid, nil, 0, nil); err == nil || errors.Is(err, syscall.ECHILD) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// serverKillGrace is how long a stopped whisper server may take to disappear;
// SIGKILL is immediate, so it only guards log flushing.
const serverKillGrace = 500 * time.Millisecond

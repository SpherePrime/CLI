//go:build windows

package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/sh/v3/interp"
)

// defaultKillTimeout matches mvdan's DefaultExecHandler default.
const defaultKillTimeout = 2 * time.Second

// createNewProcessGroup isolates the child in its own Windows process group
// so the whole tree can be addressed when the command is cancelled.
const createNewProcessGroup = 0x00000200

// isolateProcess is a no-op on Windows: process separation is arranged by
// the exec handler below, which needs the child PID to target the tree.
func isolateProcess(_ *exec.Cmd) {}

// processGroupExecHandler returns an ExecHandlerFunc that kills the whole
// process tree when the command's context is cancelled.
//
// interp.DefaultExecHandler installs its Cancel/WaitDelay pair only on
// non-Windows platforms, which leaves two failure modes that surface as a
// permanently hanging background job:
//
//   - Cancelling the context terminates the direct child only, so a
//     grandchild that inherited the stdout/stderr pipes keeps running.
//   - Without WaitDelay, cmd.Wait keeps waiting for those pipe copies to
//     finish, so it never returns even though the child is already dead.
//
// Running taskkill against the process tree releases the inherited pipes,
// and WaitDelay bounds the wait in case a descendant still holds one.
func processGroupExecHandler(killTimeout time.Duration) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		hc := interp.HandlerCtx(ctx)
		path, err := interp.LookPathDir(hc.Dir, hc.Env, args[0])
		if err != nil {
			fmt.Fprintln(hc.Stderr, err)
			return interp.ExitStatus(127)
		}

		cmd := exec.Cmd{
			Path:   path,
			Args:   args,
			Env:    execEnvList(hc.Env),
			Dir:    hc.Dir,
			Stdin:  hc.Stdin,
			Stdout: hc.Stdout,
			Stderr: hc.Stderr,
			SysProcAttr: &syscall.SysProcAttr{
				CreationFlags: createNewProcessGroup,
			},
			WaitDelay: killTimeout,
		}

		err = cmd.Start()
		if err == nil {
			stopf := context.AfterFunc(ctx, func() {
				killProcessTree(cmd.Process.Pid)
			})
			defer stopf()

			err = cmd.Wait()
		}

		return exitStatusFromError(ctx, hc.Stderr, err)
	}
}

// killProcessTree terminates the process and all of its descendants. Errors
// are ignored: taskkill also fails when the process already exited, and the
// exec handler still bounds the wait through WaitDelay.
func killProcessTree(pid int) {
	taskkill := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(pid))
	taskkill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = taskkill.Run()
}

// exitStatusFromError translates an exec error into an interp exit status,
// matching the conventions of interp.DefaultExecHandler.
func exitStatusFromError(ctx context.Context, stderr io.Writer, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		return interp.ExitStatus(1)
	}
	switch err := err.(type) {
	case *exec.ExitError:
		return interp.ExitStatus(uint8(err.ExitCode()))
	case *exec.Error:
		fmt.Fprintf(stderr, "%v\n", err)
		return interp.ExitStatus(127)
	default:
		return err
	}
}

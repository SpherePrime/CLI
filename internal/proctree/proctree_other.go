//go:build !windows

package proctree

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

type Group struct{}

func Prepare(cmd *exec.Cmd) {}

func Attach(cmd *exec.Cmd) (*Group, error) {
	return &Group{}, nil
}

func (g *Group) Kill() {}

func WaitExit(pid int) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		for {
			if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
				close(done)
				return
			}
			time.Sleep(2 * time.Second)
		}
	}()
	return done
}

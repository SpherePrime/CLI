//go:build windows
// +build windows

package cmd

import (
	"os/exec"
	"syscall"

	"github.com/dwertyfa288/CLI/vendordeps/x/sys/windows"
)

func detachProcess(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS
}

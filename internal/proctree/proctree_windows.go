//go:build windows

package proctree

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/SpherePrime/CLI/vendordeps/x/sys/windows"
)

var resumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

type Group struct {
	mu     sync.Mutex
	handle windows.Handle
}

func Prepare(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
}

func Attach(cmd *exec.Cmd) (*Group, error) {
	if cmd.Process == nil {
		return &Group{}, errors.New("process is not started")
	}
	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		_ = cmd.Process.Kill()
		return &Group{}, fmt.Errorf("open process: %w", err)
	}
	defer windows.CloseHandle(process)
	group, assignErr := newGroup(process)
	if status, _, _ := resumeProcess.Call(uintptr(process)); status != 0 {
		group.Kill()
		_ = cmd.Process.Kill()
		return &Group{}, fmt.Errorf("resume process: status 0x%x", status)
	}
	return group, assignErr
}

func newGroup(process windows.Handle) (*Group, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return &Group{}, fmt.Errorf("create job: %w", err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		windows.CloseHandle(job)
		return &Group{}, fmt.Errorf("configure job: %w", err)
	}
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(job)
		return &Group{}, fmt.Errorf("assign job: %w", err)
	}
	return &Group{handle: job}, nil
}

func (g *Group) Kill() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.handle == 0 {
		return
	}
	_ = windows.TerminateJobObject(g.handle, 1)
	_ = windows.CloseHandle(g.handle)
	g.handle = 0
}

func WaitExit(pid int) <-chan struct{} {
	done := make(chan struct{})
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		close(done)
		return done
	}
	if err != nil {
		return nil
	}
	go func() {
		defer windows.CloseHandle(handle)
		_, _ = windows.WaitForSingleObject(handle, windows.INFINITE)
		close(done)
	}()
	return done
}

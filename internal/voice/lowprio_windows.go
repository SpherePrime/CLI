package voice

import (
	"syscall"
)

// belowNormalPriorityClass is Windows' BELOW_NORMAL_PRIORITY_CLASS; the
// constant is spelled out because the windows package constants live in
// golang.org/x/sys, which this module does not import.
const belowNormalPriorityClass = 0x00004000

// lowPriorityCommand keeps the command unchanged on Windows; the priority
// class is applied to the process directly in lowPrioritySysAttr.
func lowPriorityCommand(command []string) []string {
	return command
}

// lowPrioritySysAttr returns the process attributes that start a Windows
// build below normal priority, so minutes of compiling never starve the
// desktop the user is working with.
func lowPrioritySysAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: belowNormalPriorityClass}
}

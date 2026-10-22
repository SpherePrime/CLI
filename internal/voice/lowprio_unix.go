//go:build unix

package voice

import (
	"os/exec"
	"runtime"
	"syscall"
)

// buildNiceCommand wraps a command so the process runs at the lowest CPU
// priority on unix. A whisper.cpp build takes minutes and would otherwise
// fight the desktop for cores even with a capped job count; niced it stays
// polite on a machine the user is working on. The nice binary ships with
// coreutils on every supported distribution; when it is missing the command
// simply runs un-niced.
func lowPriorityCommand(command []string) []string {
	if runtime.GOOS == "windows" || len(command) == 0 {
		return command
	}
	nice, err := exec.LookPath("nice")
	if err != nil {
		return command
	}
	return append([]string{nice, "-n", "19"}, command...)
}

// lowPrioritySysAttr leaves process attributes untouched on unix, where the
// niceness wrapper already lowers the priority.
func lowPrioritySysAttr() *syscall.SysProcAttr {
	return nil
}

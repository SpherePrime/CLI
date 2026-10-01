package tools

import (
	"fmt"
	"runtime"
)

// sleepCommand returns a shell command that blocks for roughly the given
// number of seconds, using tools the host actually has.
//
// The background-shell tests need a child that stays alive for a while, and
// they used to reach for `sleep`. Windows has no `sleep` binary and Prime's
// coreutils shim does not provide one, so those tests failed there with
// "command not found" (exit 127) instead of exercising the behaviour they
// were written for. The rest of those commands (for loops, &&) are handled by
// Prime's own shell interpreter, so only the sleep has to be swapped.
func sleepCommand(seconds float64) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(
			`powershell -NoProfile -NonInteractive -Command "Start-Sleep -Milliseconds %d"`,
			int(seconds*1000))
	}
	return fmt.Sprintf("sleep %g", seconds)
}

// sleepThenCommand returns a shell command that sleeps and then runs suffix,
// so a test can keep a job running for a while and still produce output.
func sleepThenCommand(seconds float64, suffix string) string {
	return sleepCommand(seconds) + " && " + suffix
}
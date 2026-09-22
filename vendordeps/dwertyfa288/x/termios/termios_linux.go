//go:build linux
// +build linux

package termios

import "github.com/SpherePrime/CLI/vendordeps/x/sys/unix"

const (
	ioctlGets       = unix.TCGETS
	ioctlSets       = unix.TCSETS
	ioctlGetWinSize = unix.TIOCGWINSZ
	ioctlSetWinSize = unix.TIOCSWINSZ
)

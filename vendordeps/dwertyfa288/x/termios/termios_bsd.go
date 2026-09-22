//go:build darwin || netbsd || freebsd || openbsd || dragonfly
// +build darwin netbsd freebsd openbsd dragonfly

package termios

import "github.com/SpherePrime/CLI/vendordeps/x/sys/unix"

const (
	ioctlGets       = unix.TIOCGETA
	ioctlSets       = unix.TIOCSETA
	ioctlGetWinSize = unix.TIOCGWINSZ
	ioctlSetWinSize = unix.TIOCSWINSZ
)

//go:build darwin || linux || freebsd || solaris || aix
// +build darwin linux freebsd solaris aix

package uv

import "github.com/dwertyfa288/CLI/vendordeps/x/sys/unix"

func supportsHardTabs(oflag uint64) bool {
	return oflag&unix.TABDLY == unix.TAB0
}

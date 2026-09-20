//go:build aix || linux || solaris || zos
// +build aix linux solaris zos

package term

import "github.com/dwertyfa288/CLI/vendordeps/x/sys/unix"

const (
	ioctlReadTermios  = unix.TCGETS
	ioctlWriteTermios = unix.TCSETS
)

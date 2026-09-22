//go:build aix || linux || solaris || zos
// +build aix linux solaris zos

package term

import "github.com/SpherePrime/CLI/vendordeps/x/sys/unix"

const (
	ioctlReadTermios  = unix.TCGETS
	ioctlWriteTermios = unix.TCSETS
)

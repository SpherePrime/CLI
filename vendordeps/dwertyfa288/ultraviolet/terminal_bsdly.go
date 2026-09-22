//go:build darwin || linux || aix
// +build darwin linux aix

package uv

import "github.com/SpherePrime/CLI/vendordeps/x/sys/unix"

func supportsBackspace(lflag uint64) bool {
	return lflag&unix.BSDLY == unix.BS0
}

//go:build darwin

package fsnotify

import "github.com/SpherePrime/CLI/vendordeps/x/sys/unix"

// note: this constant is not defined on BSD
const openMode = unix.O_EVTONLY | unix.O_CLOEXEC

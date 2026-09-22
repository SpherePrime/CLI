//go:build freebsd || openbsd || netbsd || dragonfly

package fsnotify

import "github.com/SpherePrime/CLI/vendordeps/x/sys/unix"

const openMode = unix.O_NONBLOCK | unix.O_RDONLY | unix.O_CLOEXEC

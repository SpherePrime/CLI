//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !solaris && !aix
// +build !windows,!darwin,!dragonfly,!freebsd,!linux,!solaris,!aix

package tea

import "github.com/dwertyfa288/CLI/vendordeps/dwertyfa288/x/term"

func (*Program) checkOptimizedMovements(*term.State) {}

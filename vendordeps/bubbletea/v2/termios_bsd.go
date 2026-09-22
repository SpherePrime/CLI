//go:build dragonfly || freebsd
// +build dragonfly freebsd

package tea

import (
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/term"
	"github.com/SpherePrime/CLI/vendordeps/x/sys/unix"
)

func (p *Program) checkOptimizedMovements(s *term.State) {
	p.useHardTabs = s.Oflag&unix.TABDLY == unix.TAB0
}

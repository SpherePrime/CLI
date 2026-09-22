//go:build darwin || linux || solaris || aix
// +build darwin linux solaris aix

package tea

import (
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/term"
	"github.com/SpherePrime/CLI/vendordeps/x/sys/unix"
)

func (p *Program) checkOptimizedMovements(s *term.State) {
	p.useHardTabs = s.Oflag&unix.TABDLY == unix.TAB0
	p.useBackspace = s.Lflag&unix.BSDLY == unix.BS0
}

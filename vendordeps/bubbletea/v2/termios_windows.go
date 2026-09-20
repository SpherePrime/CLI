//go:build windows
// +build windows

package tea

import "github.com/dwertyfa288/CLI/vendordeps/dwertyfa288/x/term"

func (p *Program) checkOptimizedMovements(*term.State) {
	p.useHardTabs = true
	p.useBackspace = true
}

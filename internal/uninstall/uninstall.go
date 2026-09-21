// Package uninstall removes the Prime binary and optionally its config
// and data directories.
package uninstall

import (
	"fmt"
	"io"
	"os"
)

// Options describes what should be removed.
type Options struct {
	Executable string
	PurgeDirs  []string
	Out        io.Writer
}

// Run removes PATH entries pointing at the binary directory, deletes the
// purge directories, and finally removes the binary itself.
func Run(opt Options) error {
	if opt.Out == nil {
		opt.Out = os.Stdout
	}
	if _, err := os.Stat(opt.Executable); err != nil {
		return fmt.Errorf("binary not found at %s: %w", opt.Executable, err)
	}

	if err := removeFromUserPath(opt.Executable); err != nil {
		fmt.Fprintf(opt.Out, "warning: could not update PATH: %v\n", err)
	}

	for _, dir := range opt.PurgeDirs {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove %s: %w", dir, err)
		}
		fmt.Fprintf(opt.Out, "removed %s\n", dir)
	}

	note, err := removeBinary(opt.Executable)
	if err != nil {
		return err
	}
	if note != "" {
		fmt.Fprintln(opt.Out, note)
		return nil
	}
	fmt.Fprintf(opt.Out, "removed %s\n", opt.Executable)
	return nil
}

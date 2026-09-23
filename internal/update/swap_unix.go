//go:build !windows

package update

import (
	"fmt"
	"os"
	"path/filepath"
)

func swapBinary(exe string, data []byte) error {
	// A running process cannot be renamed on most Unix filesystems while
	// it holds the executable open. Unlink the current binary first; the
	// inode stays alive for the running process until it exits, so a later
	// rename only has to create the new path.
	dir := filepath.Dir(exe)
	tmp := filepath.Join(dir, ".prime-update-"+filepath.Base(exe))
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := os.Remove(exe); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return fmt.Errorf("remove current binary: %w", err)
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

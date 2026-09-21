//go:build !windows

package update

import (
	"fmt"
	"os"
	"path/filepath"
)

func swapBinary(exe string, data []byte) error {
	tmp := filepath.Join(filepath.Dir(exe), ".prime-update-"+filepath.Base(exe))
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

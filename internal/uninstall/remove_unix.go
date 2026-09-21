//go:build !windows

package uninstall

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func removeFromUserPath(exe string) error {
	return nil
}

func removeBinary(exe string) (string, error) {
	if err := os.Remove(exe); err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) {
			return "", fmt.Errorf("permission denied: try sudo rm %q", exe)
		}
		return "", err
	}
	return "", nil
}

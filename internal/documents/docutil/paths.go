package docutil

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/SpherePrime/CLI/internal/filepathext"
)

func ResolvePath(workingDir, rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", errors.New("path is required")
	}
	if filepathext.SmartIsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}
	wd := strings.TrimSpace(workingDir)
	if wd == "" {
		return "", errors.New("working directory is required for relative path")
	}
	return filepath.Clean(filepathext.SmartJoin(wd, trimmed)), nil
}

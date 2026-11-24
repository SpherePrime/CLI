//go:build windows

package skills

import (
	"errors"

	"github.com/SpherePrime/CLI/vendordeps/x/sys/windows"
)

func isTransientRenameError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

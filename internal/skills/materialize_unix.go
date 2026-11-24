//go:build !windows

package skills

func isTransientRenameError(error) bool {
	return false
}

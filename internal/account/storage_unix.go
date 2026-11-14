//go:build !windows

package account

import "os"

func saveToken(path, token string) error {
	plain, err := marshalStoredToken(token)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, plain)
}

func loadToken(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", os.ErrNotExist
		}
		return "", err
	}
	return unmarshalStoredToken(raw)
}

func isTransientRenameError(error) bool {
	return false
}

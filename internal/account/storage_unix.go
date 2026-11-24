//go:build !windows

package account

import "os"

func saveStoredAccount(path string, acc storedAccount) error {
	plain, err := marshalStoredAccount(acc)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, plain)
}

func loadStoredAccount(path string) (storedAccount, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return storedAccount{}, os.ErrNotExist
		}
		return storedAccount{}, err
	}
	return unmarshalStoredAccount(raw)
}

func saveToken(path, token string) error {
	return saveStoredAccount(path, storedAccount{Token: token})
}

func loadToken(path string) (string, error) {
	acc, err := loadStoredAccount(path)
	if err != nil {
		return "", err
	}
	return acc.Token, nil
}

func isTransientRenameError(error) bool {
	return false
}

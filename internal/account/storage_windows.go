//go:build windows

package account

import (
	"errors"
	"os"
	"runtime"
	"unsafe"

	"github.com/SpherePrime/CLI/vendordeps/x/sys/windows"
)

func saveStoredAccount(path string, acc storedAccount) error {
	plain, err := marshalStoredAccount(acc)
	if err != nil {
		return err
	}
	encrypted, err := protectDPAPI(plain)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, encrypted)
}

func loadStoredAccount(path string) (storedAccount, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return storedAccount{}, os.ErrNotExist
		}
		return storedAccount{}, err
	}
	plain, err := unprotectDPAPI(raw)
	if err != nil {
		return storedAccount{}, err
	}
	return unmarshalStoredAccount(plain)
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

func protectDPAPI(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, errors.New("empty payload")
	}
	var out windows.DataBlob
	in := windows.DataBlob{
		Size: uint32(len(plain)),
		Data: unsafe.SliceData(plain),
	}
	err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	runtime.KeepAlive(plain)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))

	result := make([]byte, out.Size)
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}

func unprotectDPAPI(cipher []byte) ([]byte, error) {
	if len(cipher) == 0 {
		return nil, errors.New("empty encrypted payload")
	}
	var out windows.DataBlob
	in := windows.DataBlob{
		Size: uint32(len(cipher)),
		Data: unsafe.SliceData(cipher),
	}
	err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	runtime.KeepAlive(cipher)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))

	result := make([]byte, out.Size)
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}

func isTransientRenameError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

package account

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type storedAccount struct {
	Token string `json:"token"`
}

func marshalStoredToken(token string) ([]byte, error) {
	return json.Marshal(storedAccount{Token: token})
}

func unmarshalStoredToken(raw []byte) (string, error) {
	var payload storedAccount
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	return payload.Token, nil
}

func deleteToken(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func writeFileAtomic(path string, data []byte) error {
	cleanPath := filepath.Clean(path)
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(cleanPath)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmpFile.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmpFile.Chmod(0o600); err != nil {
		cleanup()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := renameWithRetry(tmpPath, cleanPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

const renameRetryBudget = 2 * time.Second

func renameWithRetry(tmpPath, destPath string) error {
	var slept time.Duration
	delay := time.Millisecond
	for {
		err := os.Rename(tmpPath, destPath)
		if err == nil || !isTransientRenameError(err) || slept >= renameRetryBudget {
			return err
		}
		time.Sleep(delay)
		slept += delay
		delay = min(delay*2, 50*time.Millisecond)
	}
}

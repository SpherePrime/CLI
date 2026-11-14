package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestLoginAndLogoutCmd_ValidArgsIncludePrime(t *testing.T) {
	t.Parallel()

	require.Contains(t, loginCmd.ValidArgs, "prime")
	require.Contains(t, loginCmd.ValidArgs, "account")
	require.Contains(t, logoutCmd.ValidArgs, "prime")
	require.Contains(t, logoutCmd.ValidArgs, "account")
}

func TestLoginAndLogoutPrimeAccountFlow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/me":
			if r.Header.Get("Authorization") == "Bearer tok_cli_123" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"user": map[string]string{"email": "cli@sphereprime.ru"},
				})
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		case strings.HasPrefix(r.URL.Path, "/api/auth/desktop-auth/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  "tok_cli_123",
				"user":   map[string]string{"email": "cli@sphereprime.ru"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	storagePath := filepath.Join(t.TempDir(), "prime-account.bin")
	svc := account.NewService(
		account.WithStoragePath(storagePath),
		account.WithBaseURL(srv.URL),
		account.WithPollInterval(15*time.Millisecond),
		account.WithOpenBrowser(func(string) error { return nil }),
	)
	defer svc.Close()

	var loginOut bytes.Buffer
	err := loginPrimeAccount(context.Background(), svc, &loginOut, false)
	require.NoError(t, err)
	require.Contains(t, loginOut.String(), srv.URL+"/auth/google?app_hash=desktop_")
	require.Contains(t, loginOut.String(), "cli@sphereprime.ru")

	var secondOut bytes.Buffer
	err = loginPrimeAccount(context.Background(), svc, &secondOut, false)
	require.NoError(t, err)
	require.Contains(t, secondOut.String(), "already logged in")
	require.Contains(t, secondOut.String(), "cli@sphereprime.ru")
	require.Contains(t, secondOut.String(), "--force")

	var cancelOut bytes.Buffer
	err = logoutPrimeAccount(context.Background(), svc, &cancelOut, false, func(string) (bool, error) {
		return false, nil
	})
	require.NoError(t, err)
	require.Contains(t, cancelOut.String(), "Logout cancelled")
	require.True(t, svc.Restore(context.Background()).SignedIn)

	var logoutOut bytes.Buffer
	err = logoutPrimeAccount(context.Background(), svc, &logoutOut, true, nil)
	require.NoError(t, err)
	require.Contains(t, logoutOut.String(), "Successfully logged out")
	require.False(t, svc.Restore(context.Background()).SignedIn)
	_, statErr := os.Stat(storagePath)
	require.True(t, os.IsNotExist(statErr))
}

func TestRunCmd_RequiresPrimeAccount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/me" && r.Header.Get("Authorization") == "Bearer tok_run_ok" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]string{"email": "runner@sphereprime.ru"},
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	globalDir := t.TempDir()
	t.Setenv("PRIME_GLOBAL_DATA", globalDir)
	t.Setenv("PRIME_ACCOUNT_URL", srv.URL)

	err := runCmd.RunE(runCmd, []string{"hello"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "prime login")
	require.Contains(t, err.Error(), "prime")
	require.False(t, strings.HasSuffix(err.Error(), "."))

	svc := account.NewService(
		account.WithBaseURL(srv.URL),
		account.WithPollInterval(10*time.Millisecond),
		account.WithOpenBrowser(func(string) error { return nil }),
	)
	defer svc.Close()

	checkErr := checkPrimeAccount(context.Background(), svc)
	require.Error(t, checkErr)
	require.False(t, strings.HasSuffix(checkErr.Error(), "."))
}

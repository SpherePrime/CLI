package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestStorageRoundtripAndPermissions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "prime-account.bin")
	_, err := loadToken(path)
	require.ErrorIs(t, err, os.ErrNotExist)

	const secret = "tok_super_secret_value"
	require.NoError(t, saveToken(path, secret))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		require.False(t, bytes.Contains(raw, []byte(secret)))
	} else {
		info, statErr := os.Stat(path)
		require.NoError(t, statErr)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	got, err := loadToken(path)
	require.NoError(t, err)
	require.Equal(t, secret, got)

	require.NoError(t, deleteToken(path))
	require.NoError(t, deleteToken(path))
	_, err = loadToken(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestRestoreMissingAndCorruptFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prime-account.bin")
	svc := NewService(WithStoragePath(path), WithBaseURL("http://127.0.0.1:1"))
	defer svc.Close()

	st := svc.Restore(context.Background())
	require.False(t, st.SignedIn)
	require.False(t, st.HasStoredSession)
	require.False(t, st.NetworkError)
	require.Empty(t, st.Error)

	require.NoError(t, os.WriteFile(path, []byte("not-valid-dpapi-or-json"), 0o600))
	st = svc.Restore(context.Background())
	require.False(t, st.SignedIn)
	require.False(t, st.HasStoredSession)
	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err))
}

func TestRestoreValidToken(t *testing.T) {
	t.Parallel()

	var authHeader atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/auth/me", r.URL.Path)
		authHeader.Store(r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]string{"email": "alice@sphereprime.ru"},
		})
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "prime-account.bin")
	require.NoError(t, saveToken(path, "tok_valid"))

	svc := NewService(WithStoragePath(path), WithBaseURL(srv.URL))
	defer svc.Close()

	st := svc.Restore(context.Background())
	require.True(t, st.SignedIn)
	require.True(t, st.HasStoredSession)
	require.False(t, st.NetworkError)
	require.Empty(t, st.Error)
	require.Equal(t, "alice@sphereprime.ru", st.Email)
	require.Equal(t, "Bearer tok_valid", authHeader.Load())
}

func TestRestoreUnauthorizedDeletesToken(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			path := filepath.Join(t.TempDir(), "prime-account.bin")
			require.NoError(t, saveToken(path, "tok_expired"))

			svc := NewService(WithStoragePath(path), WithBaseURL(srv.URL))
			defer svc.Close()

			st := svc.Restore(context.Background())
			require.False(t, st.SignedIn)
			require.False(t, st.HasStoredSession)
			require.False(t, st.NetworkError)
			require.NotEmpty(t, st.Error)

			_, err := os.Stat(path)
			require.True(t, os.IsNotExist(err))
		})
	}
}

func TestRestoreServerErrorAndRedirectPreserveToken(t *testing.T) {
	t.Parallel()

	t.Run("503", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		path := filepath.Join(t.TempDir(), "prime-account.bin")
		require.NoError(t, saveToken(path, "tok_keep_503"))

		svc := NewService(WithStoragePath(path), WithBaseURL(srv.URL))
		defer svc.Close()

		st := svc.Restore(context.Background())
		require.False(t, st.SignedIn)
		require.True(t, st.HasStoredSession)
		require.True(t, st.NetworkError)
		require.NotEmpty(t, st.Error)

		got, err := loadToken(path)
		require.NoError(t, err)
		require.Equal(t, "tok_keep_503", got)
	})

	t.Run("redirect rejected", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/other", http.StatusFound)
		}))
		defer srv.Close()

		path := filepath.Join(t.TempDir(), "prime-account.bin")
		require.NoError(t, saveToken(path, "tok_keep_redirect"))

		svc := NewService(WithStoragePath(path), WithBaseURL(srv.URL))
		defer svc.Close()

		st := svc.Restore(context.Background())
		require.False(t, st.SignedIn)
		require.True(t, st.HasStoredSession)
		require.True(t, st.NetworkError)

		got, err := loadToken(path)
		require.NoError(t, err)
		require.Equal(t, "tok_keep_redirect", got)
	})

	t.Run("offline", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := srv.URL
		srv.Close()

		path := filepath.Join(t.TempDir(), "prime-account.bin")
		require.NoError(t, saveToken(path, "tok_keep_offline"))

		svc := NewService(WithStoragePath(path), WithBaseURL(url))
		defer svc.Close()

		st := svc.Restore(context.Background())
		require.False(t, st.SignedIn)
		require.True(t, st.HasStoredSession)
		require.True(t, st.NetworkError)

		got, err := loadToken(path)
		require.NoError(t, err)
		require.Equal(t, "tok_keep_offline", got)
	})
}

func TestStartLoginSuccessFlow(t *testing.T) {
	t.Parallel()

	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasPrefix(r.URL.Path, "/api/auth/desktop-auth/desktop_"))
		if polls.Add(1) == 1 {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"token":  "tok_oauth_ok",
			"user":   map[string]string{"email": "oauth@sphereprime.ru"},
		})
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "prime-account.bin")
	var openedURL string
	svc := NewService(
		WithStoragePath(path),
		WithBaseURL(srv.URL),
		WithPollInterval(5*time.Millisecond),
		WithMaxAttempts(20),
		WithOpenBrowser(func(u string) error {
			openedURL = u
			return nil
		}),
	)
	defer svc.Close()

	st, err := svc.StartLogin(context.Background())
	require.NoError(t, err)
	require.True(t, st.Waiting)
	require.Empty(t, st.Error)
	require.Equal(t, openedURL, st.LoginURL)
	require.Regexp(t, regexp.MustCompile(`^`+regexp.QuoteMeta(srv.URL)+`/auth/google\?app_hash=desktop_[0-9a-f]{64}$`), st.LoginURL)

	require.Eventually(t, func() bool {
		return svc.State(context.Background()).SignedIn
	}, 2*time.Second, 5*time.Millisecond)

	final := svc.State(context.Background())
	require.True(t, final.SignedIn)
	require.False(t, final.Waiting)
	require.True(t, final.HasStoredSession)
	require.Empty(t, final.LoginURL)
	require.Equal(t, "oauth@sphereprime.ru", final.Email)

	got, err := loadToken(path)
	require.NoError(t, err)
	require.Equal(t, "tok_oauth_ok", got)
}

func TestStartLoginBrowserFailureKeepsPolling(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"token":  "tok_headless",
			"user":   map[string]string{"email": "ssh@sphereprime.ru"},
		})
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "prime-account.bin")
	svc := NewService(
		WithStoragePath(path),
		WithBaseURL(srv.URL),
		WithPollInterval(15*time.Millisecond),
		WithMaxAttempts(10),
		WithOpenBrowser(func(string) error {
			return errors.New("no display")
		}),
	)
	defer svc.Close()

	st, err := svc.StartLogin(context.Background())
	require.NoError(t, err)
	require.True(t, st.Waiting)
	require.NotEmpty(t, st.LoginURL)
	require.NotEmpty(t, st.Error)

	require.Eventually(t, func() bool {
		return svc.State(context.Background()).SignedIn
	}, 2*time.Second, 5*time.Millisecond)
	require.Empty(t, svc.State(context.Background()).Error)
}

func TestStartLogin5xxAndTimeout(t *testing.T) {
	t.Parallel()

	t.Run("5xx stops polling", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()

		svc := NewService(
			WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")),
			WithBaseURL(srv.URL),
			WithPollInterval(5*time.Millisecond),
			WithMaxAttempts(10),
			WithOpenBrowser(func(string) error { return nil }),
		)
		defer svc.Close()

		_, err := svc.StartLogin(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return !svc.State(context.Background()).Waiting
		}, 2*time.Second, 5*time.Millisecond)

		st := svc.State(context.Background())
		require.False(t, st.SignedIn)
		require.Contains(t, st.Error, "502")
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
		}))
		defer srv.Close()

		svc := NewService(
			WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")),
			WithBaseURL(srv.URL),
			WithPollInterval(5*time.Millisecond),
			WithMaxAttempts(2),
			WithOpenBrowser(func(string) error { return nil }),
		)
		defer svc.Close()

		_, err := svc.StartLogin(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return !svc.State(context.Background()).Waiting
		}, 2*time.Second, 5*time.Millisecond)

		st := svc.State(context.Background())
		require.False(t, st.SignedIn)
		require.Equal(t, errLoginTimeout, st.Error)
	})
}

func TestCancelOrLogoutAndFlowSuperseding(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	hashes := []string{}
	allowSecond := atomic.Bool{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash := strings.TrimPrefix(r.URL.Path, "/api/auth/desktop-auth/")
		mu.Lock()
		first := ""
		if len(hashes) > 0 {
			first = hashes[0]
		}
		mu.Unlock()

		if hash == first {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  "tok_first_stale",
				"user":   map[string]string{"email": "stale@example.com"},
			})
			return
		}
		if allowSecond.Load() {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  "tok_second_fresh",
				"user":   map[string]string{"email": "fresh@example.com"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "prime-account.bin")
	svc := NewService(
		WithStoragePath(path),
		WithBaseURL(srv.URL),
		WithPollInterval(10*time.Millisecond),
		WithMaxAttempts(100),
		WithOpenBrowser(func(u string) error {
			_, hash, _ := strings.Cut(u, "app_hash=")
			mu.Lock()
			hashes = append(hashes, hash)
			mu.Unlock()
			return nil
		}),
	)
	defer svc.Close()

	_, err := svc.StartLogin(context.Background())
	require.NoError(t, err)
	st2, err := svc.StartLogin(context.Background())
	require.NoError(t, err)
	require.True(t, st2.Waiting)

	time.Sleep(40 * time.Millisecond)
	mid := svc.State(context.Background())
	require.True(t, mid.Waiting)
	require.False(t, mid.SignedIn)
	require.Equal(t, st2.LoginURL, mid.LoginURL)

	allowSecond.Store(true)
	require.Eventually(t, func() bool {
		return svc.State(context.Background()).SignedIn
	}, 2*time.Second, 5*time.Millisecond)
	require.Equal(t, "fresh@example.com", svc.State(context.Background()).Email)

	loggedOut := svc.CancelOrLogout(context.Background())
	require.False(t, loggedOut.SignedIn)
	require.False(t, loggedOut.HasStoredSession)
	require.Empty(t, loggedOut.Email)
	_, statErr := os.Stat(path)
	require.True(t, os.IsNotExist(statErr))
}

func TestCrossInstanceDiskSync(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/me":
			require.Equal(t, "Bearer tok_shared", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]string{"email": "shared@sphereprime.ru"},
			})
		case strings.HasPrefix(r.URL.Path, "/api/auth/desktop-auth/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  "tok_shared",
				"user":   map[string]string{"email": "shared@sphereprime.ru"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "prime-account.bin")
	svcA := NewService(
		WithStoragePath(path),
		WithBaseURL(srv.URL),
		WithPollInterval(5*time.Millisecond),
		WithMaxAttempts(10),
		WithOpenBrowser(func(string) error { return nil }),
	)
	defer svcA.Close()

	svcB := NewService(WithStoragePath(path), WithBaseURL(srv.URL))
	defer svcB.Close()

	require.False(t, svcB.State(context.Background()).SignedIn)

	_, err := svcA.StartLogin(context.Background())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return svcA.State(context.Background()).SignedIn
	}, 2*time.Second, 5*time.Millisecond)

	synced := svcB.State(context.Background())
	require.True(t, synced.SignedIn)
	require.Equal(t, "shared@sphereprime.ru", synced.Email)

	svcA.CancelOrLogout(context.Background())
	afterLogout := svcB.State(context.Background())
	require.False(t, afterLogout.SignedIn)
	require.Empty(t, afterLogout.Email)
}

func TestEnvOverrides(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "global-data")
	t.Setenv("PRIME_GLOBAL_DATA", dataDir)
	t.Setenv("PRIME_ACCOUNT_URL", "http://127.0.0.1:8787/")

	svc := NewService()
	defer svc.Close()

	require.Equal(t, filepath.Join(dataDir, "prime-account.bin"), svc.DefaultPath())
	require.Equal(t, "http://127.0.0.1:8787", svc.baseURL)
}

func TestTokenAndStateRedaction(t *testing.T) {
	t.Parallel()

	const rawToken = "tok_secret_account_value"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/me":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]string{"email": "tok@sphereprime.ru"},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  rawToken,
				"user":   map[string]string{"email": "tok@sphereprime.ru"},
			})
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "prime-account.bin")
	svc := NewService(
		WithStoragePath(path),
		WithBaseURL(srv.URL),
		WithPollInterval(5*time.Millisecond),
		WithMaxAttempts(10),
		WithOpenBrowser(func(string) error { return nil }),
	)
	defer svc.Close()

	require.Empty(t, svc.Token())

	_, err := svc.StartLogin(context.Background())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return svc.State(context.Background()).SignedIn
	}, 2*time.Second, 5*time.Millisecond)

	require.Equal(t, rawToken, svc.Token())

	svcOther := NewService(WithStoragePath(path), WithBaseURL(srv.URL))
	defer svcOther.Close()
	require.Equal(t, rawToken, svcOther.Token())

	stateJSON, err := json.Marshal(svc.State(context.Background()))
	require.NoError(t, err)
	require.NotContains(t, string(stateJSON), rawToken)

	svc.CancelOrLogout(context.Background())
	require.Empty(t, svc.Token())
	require.Empty(t, svcOther.Token())
}

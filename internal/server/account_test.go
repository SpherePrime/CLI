package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestServerInitializesAccountService(t *testing.T) {
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())

	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	require.NotNil(t, srv.Account())
	require.NoError(t, srv.Close())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inProc := NewInProcessWithOptions(ctx, nil, "")
	require.NotNil(t, inProc.Account())
}

func TestAccountEndpointsFlowAndTokenRedaction(t *testing.T) {
	t.Parallel()

	const rawToken = "tok_never_expose_in_http_json"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/me":
			if r.Header.Get("Authorization") != "Bearer "+rawToken {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]string{"email": "server@sphereprime.ru"},
			})
		case strings.HasPrefix(r.URL.Path, "/api/auth/desktop-auth/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  rawToken,
				"user":   map[string]string{"email": "server@sphereprime.ru"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	svc := account.NewService(
		testAccountOptions(t.TempDir(), upstream.URL)...,
	)
	defer svc.Close()

	srv := &Server{account: svc}
	srv.installHandler()

	doReq := func(method, path string) (int, string, proto.AccountState) {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		srv.Handler().ServeHTTP(rec, req)
		body := rec.Body.String()
		require.NotContains(t, body, rawToken)
		var st proto.AccountState
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st))
		return rec.Code, body, st
	}

	code, _, st := doReq(http.MethodGet, "/v1/account")
	require.Equal(t, http.StatusOK, code)
	require.False(t, st.SignedIn)

	code, _, st = doReq(http.MethodPost, "/v1/account/login")
	require.Equal(t, http.StatusOK, code)
	require.True(t, st.Waiting)
	require.Contains(t, st.LoginURL, upstream.URL+"/auth/google?app_hash=desktop_")

	require.Eventually(t, func() bool {
		_, _, cur := doReq(http.MethodGet, "/v1/account")
		return cur.SignedIn
	}, 2*time.Second, 5*time.Millisecond)

	code, _, st = doReq(http.MethodPost, "/v1/account/restore")
	require.Equal(t, http.StatusOK, code)
	require.True(t, st.SignedIn)
	require.Equal(t, "server@sphereprime.ru", st.Email)

	code, _, st = doReq(http.MethodPost, "/v1/account/logout")
	require.Equal(t, http.StatusOK, code)
	require.False(t, st.SignedIn)
	require.False(t, st.HasStoredSession)
	require.Empty(t, st.Email)
}

func TestAccountRestore503PreservesSessionViaHTTP(t *testing.T) {
	t.Parallel()

	failUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/auth/desktop-auth/") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  "tok_keep_via_http",
				"user":   map[string]string{"email": "user@sphereprime.ru"},
			})
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failUpstream.Close()

	svc := account.NewService(testAccountOptions(t.TempDir(), failUpstream.URL)...)
	defer svc.Close()

	srv := &Server{account: svc}
	srv.installHandler()

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/account/login", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	require.Eventually(t, func() bool {
		r := httptest.NewRecorder()
		srv.Handler().ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/v1/account", nil))
		var st proto.AccountState
		_ = json.Unmarshal(r.Body.Bytes(), &st)
		return st.SignedIn
	}, 2*time.Second, 5*time.Millisecond)

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/account/restore", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var restored proto.AccountState
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &restored))
	require.False(t, restored.SignedIn)
	require.True(t, restored.HasStoredSession)
	require.True(t, restored.NetworkError)
	require.NotContains(t, rec.Body.String(), "tok_keep_via_http")
}

func testAccountOptions(dir, baseURL string) []account.Option {
	return []account.Option{
		account.WithStoragePath(filepath.Join(dir, "prime-account.bin")),
		account.WithBaseURL(baseURL),
		account.WithPollInterval(5 * time.Millisecond),
		account.WithMaxAttempts(20),
		account.WithOpenBrowser(func(string) error { return nil }),
	}
}

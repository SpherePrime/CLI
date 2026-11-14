package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/lsp"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/internal/skills"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	"github.com/SpherePrime/CLI/internal/workspace"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type authGateWorkspace struct {
	testWorkspace
	needsInit  bool
	agentModel workspace.AgentModel
}

func (w *authGateWorkspace) ProjectNeedsInitialization() (bool, error) {
	return w.needsInit, nil
}

func (w *authGateWorkspace) AgentModel() workspace.AgentModel {
	return w.agentModel
}

func (w *authGateWorkspace) ListSkills(context.Context) ([]skills.CatalogEntry, error) {
	return nil, nil
}

func (w *authGateWorkspace) LSPGetStates() map[string]workspace.LSPClientInfo {
	return nil
}

func (w *authGateWorkspace) LSPGetDiagnosticCounts(string) lsp.DiagnosticCounts {
	return lsp.DiagnosticCounts{}
}

func (w *authGateWorkspace) MCPGetStates() map[string]mcp.ClientInfo {
	return nil
}

func (w *authGateWorkspace) MCPPendingAuth() []mcp.PendingAuthServer {
	return nil
}

func (w *authGateWorkspace) ListUserMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *authGateWorkspace) ListAllUserMessages(context.Context) ([]message.Message, error) {
	return nil, nil
}

func (w *authGateWorkspace) CreateSession(_ context.Context, title string) (session.Session, error) {
	return session.Session{ID: "sess-auth-test", Title: title}, nil
}

func (w *authGateWorkspace) ListSessions(context.Context) ([]session.Session, error) {
	return nil, nil
}

func (w *authGateWorkspace) AgentRunShellCommand(_ context.Context, _ string, command string, _ int, _ func(string), _ bool) (proto.ShellCommandResponse, error) {
	w.runPrompts = append(w.runPrompts, "!"+command)
	return proto.ShellCommandResponse{Output: "ok", ExitCode: 0}, nil
}

func newAuthGateTestUI(t *testing.T, svc *account.Service, needsInit bool) (*UI, *authGateWorkspace) {
	t.Helper()

	cfg := &config.Config{
		Options:   &config.Options{DisabledTools: []string{}},
		Agents:    map[string]config.Agent{},
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	cfg.SetupAgents()
	cfg.NormalizeOptions()

	ws := &authGateWorkspace{
		testWorkspace: testWorkspace{cfg: cfg, agentReady: true},
		needsInit:     needsInit,
	}
	com := common.DefaultCommon(ws)
	u := NewWithAccountService(com, "", false, svc)
	u.Update(tea.WindowSizeMsg{Width: 100, Height: 34})
	return u, ws
}

func renderUIText(u *UI) string {
	scr := uv.NewScreenBuffer(u.width, u.height)
	u.Draw(scr, scr.Bounds())
	return ansi.Strip(scr.Render())
}

func runAuthCmds(u *UI, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	switch m := msg.(type) {
	case nil:
		return
	case tea.BatchMsg:
		for _, sub := range m {
			runAuthCmds(u, sub)
		}
	case authStateMsg:
		u.Update(m)
	}
}

func TestAuthGateStartsActiveWhenSignedOutAndHasNoSkip(t *testing.T) {
	t.Parallel()

	storagePath := filepath.Join(t.TempDir(), "prime-account.bin")
	svc := account.NewService(
		account.WithStoragePath(storagePath),
		account.WithBaseURL("http://127.0.0.1:1"),
	)
	t.Cleanup(svc.Close)

	u, _ := newAuthGateTestUI(t, svc, false)
	require.Equal(t, uiAuthGate, u.state)
	require.True(t, u.authDeferredInit)

	initCmd := u.Init()
	require.NotNil(t, initCmd)

	st := svc.Restore(context.Background())
	u.Update(authStateMsg{State: st, FromRestore: true})

	require.Equal(t, uiAuthGate, u.state)
	require.True(t, u.authDeferredInit)

	rendered := renderUIText(u)
	require.Contains(t, rendered, "Sign in to Prime")
	require.Contains(t, rendered, "Sign in with Google")
	require.NotContains(t, rendered, "Skip")
	require.NotContains(t, rendered, "Пропустить")
}

func TestAuthGateBlocksBypassAttempts(t *testing.T) {
	t.Parallel()

	storagePath := filepath.Join(t.TempDir(), "prime-account.bin")
	svc := account.NewService(
		account.WithStoragePath(storagePath),
		account.WithBaseURL("http://127.0.0.1:1"),
	)
	t.Cleanup(svc.Close)

	u, ws := newAuthGateTestUI(t, svc, false)
	u.Update(authStateMsg{State: account.State{SignedIn: false}, FromRestore: true})

	u.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	require.False(t, u.dialog.HasDialogs())

	u.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	require.False(t, u.dialog.HasDialogs())

	u.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.False(t, u.dialog.HasDialogs())

	u.Update(sendMessageMsg{Content: "hello"})
	require.Empty(t, ws.runPrompts)

	u.Update(loadSessionMsg{session: &session.Session{ID: "sess-1"}})
	require.Equal(t, uiAuthGate, u.state)

	require.Nil(t, u.openCommandsDialog())
	require.Nil(t, u.openModelsDialog())
	require.Nil(t, u.openSessionsDialog())
	require.Nil(t, u.sendMessage("should be blocked"))

	u.setState(uiLanding, uiFocusEditor)
	require.Equal(t, uiAuthGate, u.state)
}

func TestAuthGateTransitionsToLandingAndInitializeOnSignIn(t *testing.T) {
	t.Parallel()

	t.Run("transitions to landing", func(t *testing.T) {
		t.Parallel()
		svc := account.NewService(account.WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")))
		t.Cleanup(svc.Close)

		u, _ := newAuthGateTestUI(t, svc, false)
		u.Update(authStateMsg{State: account.State{SignedIn: true, Email: "dev@sphereprime.ru"}})

		require.Equal(t, uiLanding, u.state)
		require.Equal(t, uiFocusEditor, u.focus)
		require.False(t, u.authDeferredInit)
	})

	t.Run("transitions to initialize when project needs init", func(t *testing.T) {
		t.Parallel()
		svc := account.NewService(account.WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")))
		t.Cleanup(svc.Close)

		u, _ := newAuthGateTestUI(t, svc, true)
		u.Update(authStateMsg{State: account.State{SignedIn: true, Email: "dev@sphereprime.ru"}})

		require.Equal(t, uiInitialize, u.state)
		require.Equal(t, uiFocusEditor, u.focus)
		require.False(t, u.authDeferredInit)
	})
}

func TestAuthGateLoginCopyURLAndCancel(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
	}))
	defer srv.Close()

	var openedURL atomic.Value
	svc := account.NewService(
		account.WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")),
		account.WithBaseURL(srv.URL),
		account.WithPollInterval(50*time.Millisecond),
		account.WithOpenBrowser(func(u string) error {
			openedURL.Store(u)
			return nil
		}),
	)
	t.Cleanup(svc.Close)

	u, _ := newAuthGateTestUI(t, svc, false)
	u.Update(authStateMsg{State: account.State{SignedIn: false}, FromRestore: true})

	_, cmd := u.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	runAuthCmds(u, cmd)

	require.True(t, u.authState.Waiting)
	require.NotEmpty(t, u.authState.LoginURL)
	require.Equal(t, u.authState.LoginURL, openedURL.Load())

	rendered := renderUIText(u)
	require.Contains(t, rendered, "Copy link")
	require.Contains(t, rendered, "Open again")

	_, copyCmd := u.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	require.NotNil(t, copyCmd)
	require.True(t, u.authCopied)
	require.Contains(t, renderUIText(u), "Link copied to clipboard")

	_, cancelCmd := u.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.NotNil(t, cancelCmd)
	runAuthCmds(u, cancelCmd)
	require.False(t, u.authState.Waiting)
	require.False(t, u.authCopied)
}

func TestAuthGateFitsOnStandard80x24TerminalInWaitingState(t *testing.T) {
	t.Parallel()

	svc := account.NewService(account.WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")))
	t.Cleanup(svc.Close)

	u, _ := newAuthGateTestUI(t, svc, false)
	u.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	u.Update(authStateMsg{
		State: account.State{
			Waiting:  true,
			LoginURL: "https://vpn.sphereprime.ru/auth/google?app_hash=desktop_" + strings.Repeat("a", 64),
		},
	})

	rendered := renderUIText(u)
	require.Contains(t, rendered, "Sign in to Prime")
	require.Contains(t, rendered, "01 Google Browser")
	require.Contains(t, rendered, "02 Confirm")
	require.Contains(t, rendered, "03 Session active")
	require.Contains(t, rendered, "Cancel sign-in")
	require.Contains(t, rendered, "Copy link")
	require.Contains(t, rendered, "Open again")
	require.Contains(t, rendered, "Quit")
}

func TestAuthGateDropsStaleGenerationMessages(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/me":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]string{"email": "sync@sphereprime.ru"},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  "tok_gen",
				"user":   map[string]string{"email": "sync@sphereprime.ru"},
			})
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
	t.Cleanup(svc.Close)

	u, _ := newAuthGateTestUI(t, svc, false)
	_, err := svc.StartLogin(context.Background())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return svc.State(context.Background()).SignedIn
	}, 2*time.Second, 20*time.Millisecond)

	u.Update(authStateMsg{State: svc.State(context.Background())})
	require.Equal(t, uiLanding, u.state)

	staleSyncCmd := u.syncAccountStateCmd()
	require.NotNil(t, staleSyncCmd)
	staleMsg := staleSyncCmd()

	logoutCmd := u.logoutAccount()
	runAuthCmds(u, logoutCmd)
	require.Equal(t, uiAuthGate, u.state)
	require.False(t, u.authState.SignedIn)

	u.Update(staleMsg)
	require.Equal(t, uiAuthGate, u.state)
	require.False(t, u.authState.SignedIn)

	u.authState = account.State{Waiting: true, LoginURL: "https://vpn.sphereprime.ru/auth/google?app_hash=desktop_old"}
	staleWaitMsg := authStateMsg{
		State: account.State{Waiting: true, LoginURL: "https://vpn.sphereprime.ru/auth/google?app_hash=desktop_old"},
		Gen:   u.currentAuthGen(),
	}
	_, cancelCmd := u.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	runAuthCmds(u, cancelCmd)
	require.False(t, u.authState.Waiting)

	u.Update(staleWaitMsg)
	require.False(t, u.authState.Waiting)
	require.Empty(t, u.authState.LoginURL)
}

func TestAuthGateNetworkErrorRetryAndRelogin(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/me":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
		}
	}))
	defer srv.Close()

	svc := account.NewService(
		account.WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")),
		account.WithBaseURL(srv.URL),
		account.WithOpenBrowser(func(string) error { return nil }),
	)
	t.Cleanup(svc.Close)

	u, _ := newAuthGateTestUI(t, svc, false)
	u.Update(authStateMsg{
		State: account.State{
			HasStoredSession: true,
			NetworkError:     true,
			Error:            "server down",
		},
		FromRestore: true,
	})

	rendered := renderUIText(u)
	require.Contains(t, rendered, "Retry check")
	require.Contains(t, rendered, "Sign in again")

	_, retryCmd := u.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, retryCmd)
	require.True(t, u.authState.Checking)

	u.Update(authStateMsg{
		State: account.State{
			HasStoredSession: true,
			NetworkError:     true,
			Error:            "still down",
		},
		FromRestore: true,
	})

	_, reloginCmd := u.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.NotNil(t, reloginCmd)
	runAuthCmds(u, reloginCmd)
	require.True(t, u.authState.Waiting)
}

func TestAuthGateLogoutAndStatusEmail(t *testing.T) {
	t.Parallel()

	svc := account.NewService(account.WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")))
	t.Cleanup(svc.Close)

	u, ws := newAuthGateTestUI(t, svc, false)
	u.Update(authStateMsg{State: account.State{SignedIn: true, Email: "alice@sphereprime.ru"}})
	require.Equal(t, uiLanding, u.state)

	data := u.buildStatusData()
	require.Equal(t, "alice@sphereprime.ru", data.AccountEmail)

	statusDlg := dialog.NewStatus(u.com, data)
	scr := uv.NewScreenBuffer(90, 30)
	statusDlg.Draw(scr, scr.Bounds())
	require.Contains(t, ansi.Strip(scr.Render()), "alice@sphereprime.ru")

	u.openCommandsDialog()
	require.True(t, u.dialog.ContainsDialog(dialog.CommandsID))

	u.Update(tea.PasteMsg{Content: "logout"})
	_, logoutCmd := u.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, logoutCmd)
	runAuthCmds(u, logoutCmd)
	require.False(t, u.dialog.HasDialogs())
	require.Equal(t, uiAuthGate, u.state)
	require.Empty(t, u.authState.Email)

	u.Update(authStateMsg{State: account.State{SignedIn: true, Email: "alice@sphereprime.ru"}})
	require.Equal(t, uiLanding, u.state)

	u.bangMode = true
	u.textarea.SetValue("/logout")
	_, bangCmd := u.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, bangCmd)
	require.Equal(t, uiChat, u.state)
	require.False(t, u.bangMode)

	u.textarea.SetValue("/logout")
	_, slashCmd := u.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, slashCmd)
	runAuthCmds(u, slashCmd)
	require.Equal(t, uiAuthGate, u.state)
	_ = ws
}

func TestAuthGateDiskSyncUnlocksAndRelocks(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/me":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]string{"email": "sync@sphereprime.ru"},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  "tok_synced",
				"user":   map[string]string{"email": "sync@sphereprime.ru"},
			})
		}
	}))
	defer srv.Close()

	storagePath := filepath.Join(t.TempDir(), "prime-account.bin")
	svcDesktop := account.NewService(
		account.WithStoragePath(storagePath),
		account.WithBaseURL(srv.URL),
		account.WithPollInterval(15*time.Millisecond),
		account.WithOpenBrowser(func(string) error { return nil }),
	)
	t.Cleanup(svcDesktop.Close)

	svcCLI := account.NewService(
		account.WithStoragePath(storagePath),
		account.WithBaseURL(srv.URL),
	)
	t.Cleanup(svcCLI.Close)

	u, _ := newAuthGateTestUI(t, svcCLI, false)
	require.Equal(t, uiAuthGate, u.state)

	_, err := svcDesktop.StartLogin(context.Background())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return svcDesktop.State(context.Background()).SignedIn
	}, 2*time.Second, 20*time.Millisecond)

	_, pollCmd := u.Update(authPollTickMsg{gen: u.authPollGen})
	require.NotNil(t, pollCmd)
	runAuthCmds(u, u.syncAccountStateCmd())
	require.Equal(t, uiLanding, u.state)
	require.Equal(t, "sync@sphereprime.ru", u.authState.Email)

	svcDesktop.CancelOrLogout(context.Background())

	_, syncCmd := u.Update(authSyncTickMsg{seq: u.authSyncSeq})
	require.NotNil(t, syncCmd)
	runAuthCmds(u, u.syncAccountStateCmd())
	require.Equal(t, uiAuthGate, u.state)
}

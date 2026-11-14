package model

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
	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestPrimeAILimits_SidebarAndStatusDialog(t *testing.T) {
	t.Parallel()

	svc := account.NewService(account.WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")))
	t.Cleanup(svc.Close)

	u, _ := newAuthGateTestUI(t, svc, false)
	u.Update(authStateMsg{State: account.State{SignedIn: true, Email: "dev@sphereprime.ru"}})

	u.session = &session.Session{
		ID:    "sess-limits",
		Title: "Limits Session",
	}
	u.setState(uiChat, uiFocusEditor)

	u.Update(primeAILimitsUpdatedMsg{
		limits: &primeai.Limits{
			Available: true,
			PlanName:  "Pro",
			FiveHour: &primeai.LimitWindow{
				LimitTokens:    10000,
				Percent:        45,
				ResetInSeconds: 3720,
			},
			Weekly: &primeai.LimitWindow{
				LimitTokens:    100000,
				Percent:        12,
				ResetInSeconds: 90000,
			},
		},
	})

	u.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	rendered := renderUIText(u)
	require.Contains(t, rendered, "PrimeAI Limits")
	require.Contains(t, rendered, "Pro")
	require.Contains(t, rendered, "45%")
	require.Contains(t, rendered, "12%")

	data := u.buildStatusData()
	require.NotNil(t, data.PrimeAILimits)
	require.True(t, data.PrimeAILimits.Available)

	dlg := dialog.NewStatus(u.com, data)
	scr := uv.NewScreenBuffer(90, 35)
	dlg.Draw(scr, scr.Bounds())
	statusText := ansi.Strip(scr.Render())
	require.Contains(t, statusText, "PrimeAI Limits")
	require.Contains(t, statusText, "Pro")
	require.Contains(t, statusText, "45%")
	require.Contains(t, statusText, "12%")

	u.Update(authStateMsg{State: account.State{SignedIn: false}, FromLogout: true})
	require.Nil(t, u.primeAILimits)
}

func TestPrimeAILimits_RefreshAfterAgentTurnAndStatusOpen(t *testing.T) {
	const rawAcctToken = "tok_ui_limits"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/me":
			_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]string{"email": "ui@sphereprime.ru"}})
		case strings.HasPrefix(r.URL.Path, "/api/auth/desktop-auth/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"token":  rawAcctToken,
				"user":   map[string]string{"email": "ui@sphereprime.ru"},
			})
		case r.URL.Path == "/api/plugin/session":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":     "sess_ui",
				"expiresAt": time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case r.URL.Path == "/api/plugin/limits":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"planName": "Ultra",
				"fiveHour": map[string]any{
					"limitTokens":    1000,
					"percent":        80.0,
					"resetInSeconds": 600,
				},
				"weekly": map[string]any{
					"limitTokens":    0,
					"limitRequests":  0,
					"percent":        0,
					"resetInSeconds": 0,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	t.Setenv("PRIMEAI_URL", srv.URL)

	svc := account.NewService(
		account.WithStoragePath(filepath.Join(t.TempDir(), "prime-account.bin")),
		account.WithBaseURL(srv.URL),
		account.WithPollInterval(5*time.Millisecond),
		account.WithMaxAttempts(20),
		account.WithOpenBrowser(func(string) error { return nil }),
	)
	defer svc.Close()

	_, err := svc.StartLogin(context.Background())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return svc.Token() == rawAcctToken
	}, 2*time.Second, 5*time.Millisecond)

	u, _ := newAuthGateTestUI(t, svc, false)
	u.Update(authStateMsg{State: account.State{SignedIn: true, Email: "ui@sphereprime.ru"}})

	cmd := u.openStatusDialog()
	require.NotNil(t, cmd)
	msg := cmd()
	updated, ok := msg.(primeAILimitsUpdatedMsg)
	require.True(t, ok)
	u.Update(updated)
	require.NotNil(t, u.primeAILimits)
	require.Equal(t, "Ultra", u.primeAILimits.PlanName)
	require.Equal(t, 80.0, u.primeAILimits.FiveHour.Percent)

	statusDlg, ok := u.dialog.Dialog(dialog.StatusID).(*dialog.Status)
	require.True(t, ok)
	scr := uv.NewScreenBuffer(90, 35)
	statusDlg.Draw(scr, scr.Bounds())
	statusText := ansi.Strip(scr.Render())
	require.Contains(t, statusText, "Ultra")
	require.Contains(t, statusText, "80%")
	require.Contains(t, statusText, "Unlimited")

	turnCmd := u.handleAgentNotification(notify.Notification{Type: notify.TypeAgentFinished})
	require.NotNil(t, turnCmd)
}

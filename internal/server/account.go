package server

import (
	"net/http"
	"strings"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/internal/proto"
)

func (c *controllerV1) accountService() *account.Service {
	if c.account != nil {
		return c.account
	}
	if c.server != nil && c.server.account != nil {
		return c.server.account
	}
	return nil
}

func (c *controllerV1) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	svc := c.accountService()
	if svc == nil {
		jsonError(w, http.StatusServiceUnavailable, "account service unavailable")
		return
	}
	jsonEncode(w, svc.State(r.Context()))
}

func (c *controllerV1) handlePostAccountRestore(w http.ResponseWriter, r *http.Request) {
	svc := c.accountService()
	if svc == nil {
		jsonError(w, http.StatusServiceUnavailable, "account service unavailable")
		return
	}
	jsonEncode(w, svc.Restore(r.Context()))
}

func (c *controllerV1) handlePostAccountLogin(w http.ResponseWriter, r *http.Request) {
	svc := c.accountService()
	if svc == nil {
		jsonError(w, http.StatusServiceUnavailable, "account service unavailable")
		return
	}
	state, err := svc.StartLogin(r.Context())
	if err != nil {
		if c.server != nil {
			c.server.logError(r, "Failed to start account login", "error", err)
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonEncode(w, state)
}

func (c *controllerV1) handlePostAccountLogout(w http.ResponseWriter, r *http.Request) {
	svc := c.accountService()
	if svc == nil {
		jsonError(w, http.StatusServiceUnavailable, "account service unavailable")
		return
	}
	primeai.ClearLimits()
	jsonEncode(w, svc.CancelOrLogout(r.Context()))
}

func (c *controllerV1) handleGetAccountLimits(w http.ResponseWriter, r *http.Request) {
	svc := c.accountService()
	if svc == nil {
		jsonError(w, http.StatusServiceUnavailable, "account service unavailable")
		return
	}
	token := svc.Token()
	if token == "" {
		jsonEncode(w, proto.PrimeAILimits{Available: false})
		return
	}
	if cached, ok := primeai.CachedLimitsForToken(token); ok {
		jsonEncode(w, cached)
		return
	}
	limits, err := primeai.FetchLimitsWithToken(r.Context(), token)
	if err != nil {
		limits.Available = false
		if limits.Error == "" {
			limits.Error = strings.ReplaceAll(err.Error(), token, "[redacted]")
		}
	}
	jsonEncode(w, limits)
}

func (c *controllerV1) handlePostAccountLimitsRefresh(w http.ResponseWriter, r *http.Request) {
	svc := c.accountService()
	if svc == nil {
		jsonError(w, http.StatusServiceUnavailable, "account service unavailable")
		return
	}
	token := svc.Token()
	if token == "" {
		primeai.ClearLimits()
		jsonEncode(w, proto.PrimeAILimits{Available: false})
		return
	}
	limits, err := primeai.FetchLimitsWithToken(r.Context(), token)
	if err != nil {
		limits.Available = false
		if limits.Error == "" {
			limits.Error = strings.ReplaceAll(err.Error(), token, "[redacted]")
		}
	}
	jsonEncode(w, limits)
}

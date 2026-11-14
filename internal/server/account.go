package server

import (
	"net/http"

	"github.com/SpherePrime/CLI/internal/account"
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
	jsonEncode(w, svc.CancelOrLogout(r.Context()))
}

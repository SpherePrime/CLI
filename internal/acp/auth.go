package acp

import (
	"context"
	"fmt"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
)

const (
	AuthMethodPrimeGoogle = "prime-google"

	defaultAuthPollInterval = 100 * time.Millisecond
)

type AccountService interface {
	Restore(ctx context.Context) account.State
	StartLogin(ctx context.Context) (account.State, error)
	State(ctx context.Context) account.State
}

func AuthMethods() []AuthMethod {
	return []AuthMethod{
		{
			ID:          AuthMethodPrimeGoogle,
			Name:        "Prime Account (Google)",
			Description: "Sign in to your Prime account via Google in your browser",
		},
	}
}

func (s *Server) EnsureAuthenticated(ctx context.Context) error {
	if s.account == nil {
		return NewRPCError(
			CodeAuthRequired,
			"Authentication required. Call authenticate with methodId \"prime-google\" or run `prime login`.",
		)
	}
	st := s.account.Restore(ctx)
	if !st.SignedIn {
		return NewRPCError(
			CodeAuthRequired,
			"Authentication required. Call authenticate with methodId \"prime-google\" or run `prime login`.",
		)
	}
	return nil
}

func (s *Server) Authenticate(ctx context.Context, methodID string) error {
	if methodID != AuthMethodPrimeGoogle {
		return NewRPCError(CodeInvalidParams, fmt.Sprintf("unsupported auth method: %s", methodID))
	}
	if s.account == nil {
		return NewRPCError(CodeInternalError, "account service is not configured")
	}
	if s.account.Restore(ctx).SignedIn {
		return nil
	}

	st, err := s.account.StartLogin(ctx)
	if err != nil {
		return NewRPCError(CodeInternalError, err.Error())
	}
	if st.SignedIn {
		return nil
	}

	interval := s.authPollInterval
	if interval <= 0 {
		interval = defaultAuthPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return NewRPCError(CodeInternalError, ctx.Err().Error())
		case <-ticker.C:
			cur := s.account.State(ctx)
			if cur.SignedIn {
				return nil
			}
			if !cur.Waiting {
				if cur.Error != "" {
					return NewRPCError(CodeAuthRequired, cur.Error)
				}
				return NewRPCError(CodeAuthRequired, "authentication failed")
			}
		}
	}
}

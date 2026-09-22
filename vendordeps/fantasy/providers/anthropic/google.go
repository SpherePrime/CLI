package anthropic

import (
	"github.com/SpherePrime/CLI/vendordeps/x/oauth2"
)

type googleDummyTokenSource struct{}

func (googleDummyTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "dummy-token"}, nil
}

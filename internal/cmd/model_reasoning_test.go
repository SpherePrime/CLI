package cmd

import (
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
	"strings"
	"testing"
)

func TestCLIRejectsInvalidReasoningBeforeWriting(t *testing.T) {
	store := config.NewTestStore(&config.Config{Providers: csync.NewMapFrom(map[string]config.ProviderConfig{"p": {ID: "p", Models: []catwalk.Model{{ID: "m", CanReason: true}}}})})
	err := setModelReasoning(&cobra.Command{}, store, "p/m", "typo")
	if err == nil || !strings.Contains(err.Error(), "invalid reasoning mode") {
		t.Fatalf("invalid reasoning was not validated: %v", err)
	}
}

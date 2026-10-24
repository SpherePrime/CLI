package config_test

import (
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestShellOrchestrationOptions(t *testing.T) {
	store := loadPrimeSh(t, "option multi-agent false\noption max-concurrent-agents 4")
	require.True(t, store.Config().Options.DisableMultiAgent)
	require.Equal(t, 4, store.Config().Options.GetMaxConcurrentAgents())
}

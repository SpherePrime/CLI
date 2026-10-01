package tools

import (
	"runtime"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// TestBashDescriptionNamesThePlatform pins the tool description to the shell
// utilities that actually exist on the host. Prime shims only a subset of the
// coreutils on Windows, so a description that claims "common core utils" (or
// says nothing) has the model confidently running `sleep`, `head` or `wc` and
// collecting exit 127 from every one of them.
func TestBashDescriptionNamesThePlatform(t *testing.T) {
	t.Parallel()

	desc := bashDescription(&config.Attribution{}, "test/model")

	require.Contains(t, desc, "Platform: "+runtime.GOOS,
		"the description must state the platform it is running on")

	if runtime.GOOS == "windows" {
		// The shimmed set, and the common gaps, must both be spelled out.
		for _, have := range []string{"cat", "find", "ls", "mktemp", "tar"} {
			require.Contains(t, desc, have)
		}
		for _, missing := range []string{"sleep", "head", "tail", "wc", "grep"} {
			require.Contains(t, desc, missing,
				"windows guidance must call out that %q is unavailable", missing)
		}
		require.Contains(t, desc, "run_in_background",
			"windows guidance must offer the portable alternative to sleeping")
	} else {
		require.Contains(t, desc, "POSIX toolchain",
			"unix hosts should be told the usual utilities are present")
	}
}

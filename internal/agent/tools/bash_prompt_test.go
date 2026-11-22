package tools

import (
	"bytes"
	"runtime"
	"strings"
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

func TestCoreToolDescriptionsAlignedAndBranded(t *testing.T) {
	t.Parallel()

	var bashWindows bytes.Buffer
	require.NoError(t, bashDescriptionTpl.Execute(&bashWindows, bashDescriptionData{
		BannedCommands:  "curl, sudo",
		MaxOutputLength: MaxOutputLength,
		RgAvailable:     true,
		GhAvailable:     true,
		SudoAvailable:   true,
		Platform:        "windows",
	}))

	var bashLinux bytes.Buffer
	require.NoError(t, bashDescriptionTpl.Execute(&bashLinux, bashDescriptionData{
		BannedCommands:  "curl, sudo",
		MaxOutputLength: MaxOutputLength,
		RgAvailable:     false,
		GhAvailable:     false,
		SudoAvailable:   false,
		Platform:        "linux",
	}))

	var fetchWithGH bytes.Buffer
	require.NoError(t, fetchDescriptionTpl.Execute(&fetchWithGH, fetchDescriptionData{
		GhAvailable:    true,
		MaxFetchSizeKB: MaxFetchSize / 1024,
	}))

	var webFetchWithGH bytes.Buffer
	require.NoError(t, webFetchDescriptionTpl.Execute(&webFetchWithGH, toolDescriptionData{
		GhAvailable: true,
	}))

	descriptions := map[string]string{
		"bash_windows":        bashWindows.String(),
		"bash_linux":          bashLinux.String(),
		"view":                viewDescription(),
		"edit":                editDescription,
		"multiedit":           multieditDescription,
		"write":               writeDescription,
		"glob":                globDescription(),
		"grep":                grepDescription(),
		"ls":                  lsDescription(),
		"todos":               todosDescription,
		"question":            questionDescription,
		"web_search_provider": webSearchDescription(true, false),
		"web_search_ddg":      webSearchDescription(false, true),
		"web_fetch":           webFetchWithGH.String(),
		"fetch":               fetchWithGH.String(),
	}

	forbiddenSubstrings := []string{
		"claude",
		"anthropic",
		"todowrite",
		"askuserquestion",
		"security check",
		"git safety protocol",
		"credentials.json",
		".env",
		"sensitive info",
		"sensitive information",
		"ssrf",
		"private ip",
		"private url",
	}

	for name, desc := range descriptions {
		require.NotEmpty(t, desc, name)
		lower := strings.ToLower(desc)
		for _, forbidden := range forbiddenSubstrings {
			require.NotContains(t, lower, forbidden, "tool %s contains forbidden term %q", name, forbidden)
		}
	}

	require.Contains(t, descriptions["bash_windows"], "mvdan/sh")
	require.Contains(t, descriptions["bash_windows"], "job_output")
	require.Contains(t, descriptions["bash_windows"], "job_kill")
	require.Contains(t, descriptions["bash_windows"], "gh pr create")
	require.Contains(t, descriptions["bash_windows"], "--no-verify")
	require.Contains(t, descriptions["bash_windows"], "NEW commit")

	require.Contains(t, descriptions["view"], "SKILL.md")
	require.Contains(t, descriptions["view"], "prime://skills/...")
	require.Contains(t, descriptions["view"], "200")
	require.Contains(t, descriptions["view"], "200KB")
	require.Contains(t, descriptions["view"], "ls")

	require.Contains(t, descriptions["edit"], "lsp_replace_symbol")
	require.Contains(t, descriptions["edit"], "lsp_rename")
	require.Contains(t, descriptions["edit"], "replace_all")
	require.Contains(t, descriptions["multiedit"], "multiedit")
	require.Contains(t, descriptions["multiedit"], "lsp_replace_symbol")
	require.Contains(t, descriptions["write"], "view")
	require.Contains(t, descriptions["write"], "README")

	require.Contains(t, descriptions["glob"], "100")
	require.Contains(t, descriptions["glob"], ".gitignore")
	require.Contains(t, descriptions["grep"], "100")
	require.Contains(t, descriptions["grep"], ".gitignore")
	require.Contains(t, descriptions["ls"], "1000")
	require.Contains(t, descriptions["ls"], ".gitignore")

	require.Contains(t, descriptions["todos"], "pending")
	require.Contains(t, descriptions["todos"], "in_progress")
	require.Contains(t, descriptions["todos"], "completed")
	require.Contains(t, descriptions["todos"], "active_form")

	require.Contains(t, descriptions["question"], "plan")
	require.Contains(t, descriptions["question"], "confirm_title")
	require.Contains(t, descriptions["question"], "choices")

	require.Contains(t, descriptions["web_search_provider"], "Sources:")
	require.Contains(t, descriptions["web_search_ddg"], "Sources:")
	require.Contains(t, descriptions["web_fetch"], "gh")
	require.Contains(t, descriptions["fetch"], "100KB")
}

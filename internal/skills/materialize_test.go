package skills

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestPrimeDesignerIsBuiltin(t *testing.T) {
	t.Parallel()

	var designer *Skill
	for _, skill := range DiscoverBuiltin() {
		if skill.Name == "prime-designer" {
			designer = skill
		}
	}
	require.NotNil(t, designer)
	require.Equal(t, "prime://skills/prime-designer/SKILL.md", designer.SkillFilePath)
	require.Contains(t, designer.Description, "дизайн")
	require.Contains(t, designer.Instructions, "<skill_directory>")
	require.True(t, BuiltinHasResources("prime-designer"))
	require.False(t, BuiltinHasResources("prime-config"))
}

func TestBuiltinSkillName(t *testing.T) {
	t.Parallel()

	name, ok := BuiltinSkillName("prime://skills/prime-designer/SKILL.md")
	require.True(t, ok)
	require.Equal(t, "prime-designer", name)
	for _, location := range []string{"prime://skills/prime-designer/references/quality-gate.md", "/home/user/skills/x/SKILL.md", "prime://skills/SKILL.md"} {
		_, ok := BuiltinSkillName(location)
		require.False(t, ok, location)
	}
}

func TestMaterializeBuiltinCopiesAndRefreshes(t *testing.T) {
	t.Parallel()

	for _, removed := range []string{
		"builtin/prime-designer/assets/references/README.md",
		"builtin/prime-designer/assets/examples/theme-showcase.pdf",
	} {
		_, err := builtinFS.ReadFile(removed)
		require.ErrorIs(t, err, fs.ErrNotExist, removed)
	}

	root := t.TempDir()
	directory, err := MaterializeBuiltin("prime-designer", root)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "prime-designer"), directory)

	for _, relative := range []string{"SKILL.md", "scripts/audit.mjs", "scripts/package.json", "references/quality-gate.md", "assets/fonts/Lora-Regular.ttf", "assets/themes/ocean-depths.md"} {
		embedded, err := builtinFS.ReadFile("builtin/prime-designer/" + relative)
		require.NoError(t, err, relative)
		onDisk, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(relative)))
		require.NoError(t, err, relative)
		require.Equal(t, embedded, onDisk, relative)
	}

	audit := filepath.Join(directory, "scripts", "audit.mjs")
	require.NoError(t, os.WriteFile(audit, []byte("changed"), 0o644))
	_, err = MaterializeBuiltin("prime-designer", root)
	require.NoError(t, err)
	unchanged, err := os.ReadFile(audit)
	require.NoError(t, err)
	require.Equal(t, "changed", string(unchanged))

	staleReferences := filepath.Join(directory, "assets", "references")
	require.NoError(t, os.MkdirAll(filepath.Join(staleReferences, "apex"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staleReferences, "README.md"), []byte("stale"), 0o644))
	staleExamples := filepath.Join(directory, "assets", "examples")
	require.NoError(t, os.MkdirAll(staleExamples, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staleExamples, "theme-showcase.pdf"), []byte("stale"), 0o644))
	nodeModulesFile := filepath.Join(directory, "scripts", "node_modules", "playwright", "index.js")
	require.NoError(t, os.MkdirAll(filepath.Dir(nodeModulesFile), 0o755))
	require.NoError(t, os.WriteFile(nodeModulesFile, []byte("kept"), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(directory, materializedStampFile), []byte("old-version"), 0o644))
	_, err = MaterializeBuiltin("prime-designer", root)
	require.NoError(t, err)
	refreshed, err := os.ReadFile(audit)
	require.NoError(t, err)
	embedded, err := builtinFS.ReadFile("builtin/prime-designer/scripts/audit.mjs")
	require.NoError(t, err)
	require.Equal(t, embedded, refreshed)

	_, err = os.Stat(staleReferences)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(staleExamples)
	require.ErrorIs(t, err, os.ErrNotExist)
	preserved, err := os.ReadFile(nodeModulesFile)
	require.NoError(t, err)
	require.Equal(t, "kept", string(preserved))
}

func TestDiscoverSkipsMaterializedBuiltinCopies(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	_, err := MaterializeBuiltin("prime-designer", root)
	require.NoError(t, err)
	userSkill := filepath.Join(root, "my-skill")
	require.NoError(t, os.MkdirAll(userSkill, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(userSkill, SkillFileName), []byte("---\nname: my-skill\ndescription: Mine.\n---\nBody\n"), 0o644))

	discovered := Discover([]string{root})
	require.Len(t, discovered, 1)
	require.Equal(t, "my-skill", discovered[0].Name)
}

func TestPrimeGameDevBuiltinHiddenAndMaterializes(t *testing.T) {
	t.Parallel()

	var gameDev *Skill
	builtins := DiscoverBuiltin()
	for _, skill := range builtins {
		if skill.Name == "prime-gamedev" {
			gameDev = skill
		}
	}
	require.NotNil(t, gameDev)
	require.Equal(t, "prime://skills/prime-gamedev/SKILL.md", gameDev.SkillFilePath)
	require.True(t, gameDev.DisableModelInvocation)
	require.False(t, gameDev.UserInvocable)
	require.True(t, BuiltinHasResources("prime-gamedev"))

	xml := ToPromptXML(builtins)
	require.NotContains(t, xml, "prime-gamedev")

	root := t.TempDir()
	directory, err := MaterializeBuiltin("prime-gamedev", root)
	require.NoError(t, err)
	for _, relative := range []string{
		"SKILL.md",
		"scripts/package.json",
		"scripts/extract_frames.mjs",
		"scripts/web_bot_runner.mjs",
		"scripts/native_bot_runner.mjs",
		"scripts/element_capture.mjs",
		"scripts/library.mjs",
		"scripts/presets/showcase_capture_probe.mjs",
		"scripts/presets/teleport_collision_probe.mjs",
		"scripts/presets/weapon_crosshair_probe.mjs",
		"scripts/presets/physics_bounds_probe.mjs",
		"scripts/presets/balance_and_state_probe.mjs",
		"scripts/lib/args.mjs",
		"scripts/lib/ffmpeg.mjs",
		"scripts/lib/playwright_loader.mjs",
		"scripts/lib/telemetry.mjs",
		"scripts/lib/window_target.mjs",
	} {
		embedded, err := builtinFS.ReadFile("builtin/prime-gamedev/" + relative)
		require.NoError(t, err, relative)
		onDisk, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(relative)))
		require.NoError(t, err, relative)
		require.Equal(t, embedded, onDisk, relative)
	}
}


package skills

import (
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

	require.NoError(t, os.WriteFile(filepath.Join(directory, materializedStampFile), []byte("old-version"), 0o644))
	_, err = MaterializeBuiltin("prime-designer", root)
	require.NoError(t, err)
	refreshed, err := os.ReadFile(audit)
	require.NoError(t, err)
	embedded, err := builtinFS.ReadFile("builtin/prime-designer/scripts/audit.mjs")
	require.NoError(t, err)
	require.Equal(t, embedded, refreshed)
}

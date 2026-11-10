package prompt

import (
	"bytes"
	"os"
	"testing"
	"text/template"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestCorePromptUsesLazyInstructions(t *testing.T) {
	body, err := os.ReadFile("../templates/coder.md.tpl")
	require.NoError(t, err)
	tmpl, err := template.New("coder").Parse(string(body))
	require.NoError(t, err)
	var output bytes.Buffer
	err = tmpl.Execute(&output, map[string]any{
		"Config": config.Config{}, "SmartTools": false, "InstructionTools": true,
		"WorkingDir": "/project", "Platform": "windows", "Date": "10/8/2026", "IsGitRepo": false,
		"GitStatus": "", "AvailSkillXML": "", "ContextFiles": nil, "GlobalContextFiles": nil,
	})
	require.NoError(t, err)
	require.Contains(t, output.String(), "<task_instructions>")
	require.Contains(t, output.String(), "read_instruction")
	require.Less(t, output.Len(), 12000)
}

func TestDesignRuleNeedsAvailableDesignSkill(t *testing.T) {
	body, err := os.ReadFile("../templates/coder.md.tpl")
	require.NoError(t, err)
	tmpl, err := template.New("coder").Parse(string(body))
	require.NoError(t, err)
	render := func(designSkill string) string {
		var output bytes.Buffer
		require.NoError(t, tmpl.Execute(&output, map[string]any{
			"Config": config.Config{}, "WorkingDir": "/project", "Platform": "windows", "Date": "10/8/2026",
			"DesignSkill": designSkill,
		}))
		return output.String()
	}
	require.Contains(t, render("prime://skills/prime-designer/SKILL.md"), "read prime://skills/prime-designer/SKILL.md before planning")
	require.NotContains(t, render(""), "counts as a design task")
}

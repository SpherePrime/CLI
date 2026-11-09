package prompt

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/skills"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestDesignSkillLocation(t *testing.T) {
	designer := &skills.Skill{Name: "prime-designer", SkillFilePath: "prime://skills/prime-designer/SKILL.md"}
	require.Equal(t, designer.SkillFilePath, designSkillLocation([]*skills.Skill{{Name: "jq"}, designer}))
	require.Empty(t, designSkillLocation([]*skills.Skill{{Name: "jq"}}))
	hidden := &skills.Skill{Name: "prime-designer", SkillFilePath: "x", DisableModelInvocation: true}
	require.Empty(t, designSkillLocation([]*skills.Skill{hidden}))
}

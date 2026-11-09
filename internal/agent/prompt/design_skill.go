package prompt

import "github.com/SpherePrime/CLI/internal/skills"

const designSkillName = "prime-designer"

func designSkillLocation(available []*skills.Skill) string {
	for _, skill := range available {
		if skill.Name == designSkillName && !skill.DisableModelInvocation {
			return skill.SkillFilePath
		}
	}
	return ""
}

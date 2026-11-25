import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { IntegrationSettings } from '../../src/renderer/src/features/settings/IntegrationSettings'
import {
  filterVisibleSettingsSkills,
  isBuiltinSkill
} from '../../src/renderer/src/features/settings/skillsFilter'

function makeWorkspace(skills: unknown[], disabledSkills: string[] = []) {
  return {
    busy: false,
    project: { id: 'ws-1', path: 'C:/work/demo', kind: 'project' },
    config: {
      mcp: {},
      lsp: {},
      options: {
        disabled_skills: disabledSkills,
        skills_paths: []
      }
    },
    resources: {
      mcp: {},
      lsp: {},
      skills
    },
    call: vi.fn(),
    refresh: vi.fn()
  } as any
}

describe('Settings Skills filtering', () => {
  it('identifies Prime builtin skills and keeps user/project skills', () => {
    expect(
      isBuiltinSkill({
        id: 'prime://skills/prime-designer/SKILL.md',
        name: 'prime-designer',
        source: 'system',
        label: 'system:prime-designer'
      })
    ).toBe(true)
    expect(isBuiltinSkill({ name: 'prime-gamedev' })).toBe(true)
    expect(isBuiltinSkill({ name: 'prime-pdf' })).toBe(true)
    expect(isBuiltinSkill({ name: 'prime-docx' })).toBe(true)
    expect(isBuiltinSkill({ name: 'prime-xlsx' })).toBe(true)
    expect(isBuiltinSkill({ name: 'prime-pptx' })).toBe(true)
    expect(isBuiltinSkill({ name: 'jq', source: 'system' })).toBe(true)
    expect(
      isBuiltinSkill({
        id: 'C:/Users/me/.config/prime/skills/my-skill/SKILL.md',
        name: 'my-skill',
        source: 'user',
        label: 'user:my-skill'
      })
    ).toBe(false)
    expect(
      isBuiltinSkill({
        id: 'C:/work/repo/.prime/skills/project-skill/SKILL.md',
        name: 'project-skill',
        source: 'project',
        label: 'project:project-skill'
      })
    ).toBe(false)
  })

  it('hides all builtin Prime skills from Settings -> Skills while rendering user skills', () => {
    const runtimeSkills = [
      {
        id: 'prime://skills/jq/SKILL.md',
        name: 'jq',
        description: 'JSON query helper',
        source: 'system',
        label: 'system:jq'
      },
      {
        id: 'prime://skills/prime-config/SKILL.md',
        name: 'prime-config',
        description: 'Configure Prime',
        source: 'system',
        label: 'system:prime-config'
      },
      {
        id: 'prime://skills/prime-designer/SKILL.md',
        name: 'prime-designer',
        description: 'Design system skill',
        source: 'system',
        label: 'system:prime-designer'
      },
      {
        id: 'prime://skills/prime-gamedev/SKILL.md',
        name: 'prime-gamedev',
        description: 'GameDev QA skill',
        source: 'system',
        label: 'system:prime-gamedev'
      },
      {
        id: 'prime://skills/prime-hooks/SKILL.md',
        name: 'prime-hooks',
        description: 'Hooks skill',
        source: 'system',
        label: 'system:prime-hooks'
      },
      {
        id: 'prime://skills/prime-pdf/SKILL.md',
        name: 'prime-pdf',
        description: 'PDF document skill',
        source: 'system',
        label: 'system:prime-pdf'
      },
      {
        id: 'prime://skills/prime-docx/SKILL.md',
        name: 'prime-docx',
        description: 'Word document skill',
        source: 'system',
        label: 'system:prime-docx'
      },
      {
        id: 'prime://skills/prime-xlsx/SKILL.md',
        name: 'prime-xlsx',
        description: 'Excel workbook skill',
        source: 'system',
        label: 'system:prime-xlsx'
      },
      {
        id: 'prime://skills/prime-pptx/SKILL.md',
        name: 'prime-pptx',
        description: 'PowerPoint presentation skill',
        source: 'system',
        label: 'system:prime-pptx'
      },
      {
        id: 'C:/Users/me/.config/prime/skills/brainstorming/SKILL.md',
        name: 'brainstorming',
        description: 'User brainstorming skill',
        source: 'user',
        label: 'user:brainstorming'
      }
    ]

    const visible = filterVisibleSettingsSkills(runtimeSkills, [
      'prime-designer',
      'prime-pdf',
      'prime-docx',
      'prime-xlsx',
      'prime-pptx',
      'disabled-user-skill'
    ])
    expect(visible.map((item) => item.name)).toEqual(['brainstorming', 'disabled-user-skill'])

    const visibleWithoutRuntimeEntries = filterVisibleSettingsSkills(
      [{ name: 'prime-pdf' }, { name: 'prime-docx' }, { name: 'prime-xlsx' }, { name: 'prime-pptx' }],
      ['prime-pdf', 'prime-docx', 'prime-xlsx', 'prime-pptx', 'disabled-user-skill']
    )
    expect(visibleWithoutRuntimeEntries.map((item) => item.name)).toEqual(['disabled-user-skill'])

    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <IntegrationSettings
          workspace={makeWorkspace(runtimeSkills, [
            'prime-config',
            'prime-pdf',
            'prime-docx',
            'prime-xlsx',
            'prime-pptx',
            'disabled-user-skill'
          ])}
          kind="skills"
        />
      </I18nProvider>
    )

    expect(html).not.toContain('prime-config')
    expect(html).not.toContain('prime-designer')
    expect(html).not.toContain('prime-gamedev')
    expect(html).not.toContain('prime-hooks')
    expect(html).not.toContain('prime-pdf')
    expect(html).not.toContain('prime-docx')
    expect(html).not.toContain('prime-xlsx')
    expect(html).not.toContain('prime-pptx')
    expect(html).not.toContain('>jq<')
    expect(html).toContain('brainstorming')
    expect(html).toContain('disabled-user-skill')
  })
})

import { describe, expect, it } from 'vitest'
import { hidesToolOutput } from '../../src/renderer/src/features/chat/hiddenToolOutput'

describe('hidesToolOutput', () => {
  it('hides internal tool documentation and instructions', () => {
    expect(hidesToolOutput('read_tool_documentation', { tool_name: 'agent' })).toBe(true)
    expect(hidesToolOutput('read_instruction', { id: 'verification' })).toBe(true)
  })

  it('hides skill files read with view', () => {
    expect(hidesToolOutput('view', { file_path: 'prime://skills/prime-designer/SKILL.md' })).toBe(true)
    expect(hidesToolOutput('view', { file_path: 'C:\\Users\\me\\.prime\\skills\\deploy\\SKILL.md' })).toBe(true)
    expect(hidesToolOutput('view', { file_path: 'skills/brainstorming/references/notes.md' })).toBe(true)
  })

  it('keeps project file output visible', () => {
    expect(hidesToolOutput('view', { file_path: 'src/app.ts' })).toBe(false)
    expect(hidesToolOutput('bash', { command: 'cat skills/x/SKILL.md' })).toBe(false)
    expect(hidesToolOutput('search_skills', { query: 'deploy' })).toBe(false)
  })
})

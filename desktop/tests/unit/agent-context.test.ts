import { describe, expect, it } from 'vitest'
import {
  buildAgentPrompt,
  emptyAgentContext,
  mergeContextSource
} from '../../src/shared/agentContext'

describe('agent working context', () => {
  it('rejects oversized unsaved selections instead of silently losing the tail', () => {
    expect(() =>
      mergeContextSource(emptyAgentContext(), {
        id: 'selection:large',
        kind: 'selection',
        path: 'app.ts',
        content: 'x'.repeat(12001)
      })
    ).toThrow('12000')
  })
  it('refreshes pinned files before each send and includes the task criteria', async () => {
    const context = {
      ...emptyAgentContext(),
      enabled: true,
      goal: 'Fix login',
      constraints: 'Keep the public API',
      criteria: 'Existing users can sign in',
      sources: [{ id: 'file:src/login.ts', kind: 'file' as const, path: 'src/login.ts' }]
    }
    let content = 'export const fixed = false'
    const read = async () => ({ content })
    expect(await buildAgentPrompt('Start', context, read)).toContain('export const fixed = false')
    content = 'export const fixed = true'
    const prompt = await buildAgentPrompt('Start', context, read)
    expect(prompt).toContain('export const fixed = true')
    expect(prompt).toContain('Keep the public API')
    expect(prompt).toContain('Existing users can sign in')
    expect(prompt.startsWith('Start')).toBe(true)
  })

  it('does not read files when disabled or intercept explicit shell commands', async () => {
    const read = async () => {
      throw new Error('must not read')
    }
    expect(await buildAgentPrompt('Hello', emptyAgentContext(), read)).toBe('Hello')
    expect(
      await buildAgentPrompt(
        '!git status',
        { ...emptyAgentContext(), enabled: true, goal: 'Other task' },
        read
      )
    ).toBe('!git status')
  })

  it('keeps the conversation usable when an agent-managed source disappears', async () => {
    const context = {
      ...emptyAgentContext(),
      enabled: true,
      sources: [
        { id: 'file:missing.ts', kind: 'file' as const, path: 'missing.ts' },
        { id: 'file:existing.ts', kind: 'file' as const, path: 'existing.ts' }
      ]
    }
    const prompt = await buildAgentPrompt('Continue', context, async (path) => {
      if (path === 'missing.ts') throw new Error('File missing')
      return { content: 'current project content' }
    })
    expect(prompt).toContain('Continue')
    expect(prompt).toContain('Source: missing.ts (unavailable; update the working context sources)')
    expect(prompt).toContain('current project content')
  })

  it('preserves unsaved selection snapshots, deduplicates sources and limits payload size', async () => {
    const source = {
      id: 'selection:app.ts:2:3',
      kind: 'selection' as const,
      path: 'app.ts',
      startLine: 2,
      endLine: 3,
      content: 'unsaved selection'
    }
    const updated = mergeContextSource(mergeContextSource(emptyAgentContext(), source), {
      ...source,
      content: 'updated selection'
    })
    expect(updated.sources).toHaveLength(1)
    const prompt = await buildAgentPrompt('Review', updated, async () => {
      throw new Error('must not read')
    })
    expect(prompt).toContain('updated selection')
    expect(prompt).toContain('app.ts:2-3')
    const large = mergeContextSource(emptyAgentContext(), {
      id: 'file:big.ts',
      kind: 'file',
      path: 'big.ts'
    })
    expect(
      (await buildAgentPrompt('Review', large, async () => ({ content: 'x'.repeat(90000) }))).length
    ).toBeLessThan(14000)
  })
})

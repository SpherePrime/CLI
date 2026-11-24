import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { ToolExecutionGroup } from '../../src/renderer/src/features/chat/ToolExecutionGroup'
import { ToolCard } from '../../src/renderer/src/features/chat/ToolCard'
import { computeEditDiff, extractToolDiffStats, tokenizeCodeLine } from '../../src/renderer/src/features/chat/toolDiff'
import { mergeConsecutiveToolMessages, partitionMessageParts } from '../../src/renderer/src/features/chat/toolGroupMerge'
import { parseToolOutput } from '../../src/renderer/src/features/chat/toolOutputParser'
import type { Message } from '../../src/shared/contracts'

function makeMsg(id: string, parts: Message['parts']): Message {
  return { id, session_id: 's1', role: 'assistant', parts }
}

describe('toolDiff and toolOutputParser', () => {
  it('computes line-level diff stats and syntax tokens', () => {
    const diff = computeEditDiff('const a = 1\nconst b = 2', 'const a = 1\nconst b = 3\nreturn b')
    expect(diff.added).toBe(2)
    expect(diff.removed).toBe(1)
    expect(diff.lines.some((line) => line.kind === 'del' && line.text.includes('= 2'))).toBe(true)
    expect(diff.lines.some((line) => line.kind === 'add' && line.text.includes('return b'))).toBe(true)

    const tokens = tokenizeCodeLine("const value = compute('ok', 42)")
    expect(tokens.some((t) => t.kind === 'kw' && t.text === 'const')).toBe(true)
    expect(tokens.some((t) => t.kind === 'fn' && t.text === 'compute')).toBe(true)
    expect(tokens.some((t) => t.kind === 'str' && t.text === "'ok'")).toBe(true)
    expect(tokens.some((t) => t.kind === 'num' && t.text === '42')).toBe(true)
  })

  it('extracts stats for edit, multiedit, and write', () => {
    expect(extractToolDiffStats('edit', { old_string: 'a', new_string: 'b\nc' })).toEqual({
      added: 2,
      removed: 1
    })
    expect(extractToolDiffStats('write', { content: 'line1\nline2\nline3' })).toEqual({
      added: 3,
      removed: 0
    })
  })

  it('strips result/file/cwd wrappers and separates diagnostics', () => {
    const parsed = parseToolOutput(
      '<result>\nContent replaced in file: demo.ts\n</result>\n<cwd>/repo/desktop</cwd>\n<project_diagnostics>\nHint: unused\n</project_diagnostics>\n<diagnostic_summary>\nCurrent file: 0 errors\n</diagnostic_summary>'
    )
    expect(parsed.cleanText).toBe('Content replaced in file: demo.ts')
    expect(parsed.cwd).toBe('/repo/desktop')
    expect(parsed.diagnostics).toBe('Hint: unused')
    expect(parsed.diagnosticSummary).toBe('Current file: 0 errors')
  })
})

describe('ToolExecutionGroup and consecutive tool message merging', () => {
  it('merges adjacent tool-only assistant turns into one execution group', () => {
    const merged = mergeConsecutiveToolMessages([
      makeMsg('a1', [
        { type: 'tool_call', data: { id: 'c1', name: 'edit', input: '{}', result: { content: 'ok' } } },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]),
      makeMsg('a2', [
        { type: 'tool_call', data: { id: 'c2', name: 'bash', input: '{}', result: { content: 'ok' } } },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]),
      makeMsg('a3', [{ type: 'text', data: { text: 'Done!' } }])
    ])
    expect(merged).toHaveLength(2)
    expect(merged[0].parts.filter((p) => p.type === 'tool_call')).toHaveLength(2)

    const segments = partitionMessageParts(merged[0].parts)
    expect(segments[0].kind).toBe('tool-group')
  })

  it('renders grouped tree summary with diff stats, background badge, and job_output id', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolExecutionGroup
          messageFinished
          parts={[
            {
              type: 'tool_call',
              data: {
                id: 'c1',
                name: 'edit',
                input: JSON.stringify({
                  file_path: 'src/app.ts',
                  old_string: 'old',
                  new_string: 'new1\nnew2'
                }),
                result: { content: 'ok', is_error: false }
              }
            },
            {
              type: 'tool_call',
              data: {
                id: 'c2',
                name: 'job_output',
                input: JSON.stringify({ shell_id: '016', wait: true }),
                result: { content: 'Status: completed', is_error: false }
              }
            }
          ]}
        />
      </I18nProvider>
    )
    expect(html).toContain('tool-exec-group')
    expect(html).toContain('Выполнено 2 действия')
    expect(html).toContain('+2')
    expect(html).toContain('−1')
    expect(html).toContain('tool-tree-node')
    expect(html).toContain('#016')
  })

  it('renders standalone ToolCard with diff badge on summary', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolCard
          part={{
            type: 'tool_call',
            data: {
              id: 'c1',
              name: 'edit',
              input: JSON.stringify({
                file_path: 'src/demo.ts',
                old_string: 'a\nb',
                new_string: 'a\nc\nd'
              }),
              result: { content: 'ok', is_error: false }
            }
          }}
        />
      </I18nProvider>
    )
    expect(html).toContain('tool-diff-pill')
    expect(html).toContain('+2')
    expect(html).toContain('−1')
  })
})

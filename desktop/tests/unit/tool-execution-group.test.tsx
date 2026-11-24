import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { ToolExecutionGroup } from '../../src/renderer/src/features/chat/ToolExecutionGroup'
import { ToolCard } from '../../src/renderer/src/features/chat/ToolCard'
import { ToolBody } from '../../src/renderer/src/features/chat/ToolBody'
import { computeEditDiff, extractToolDiffStats, tokenizeCodeLine } from '../../src/renderer/src/features/chat/toolDiff'
import { mergeConsecutiveToolMessages, partitionMessageParts } from '../../src/renderer/src/features/chat/toolGroupMerge'
import { classifyToolCategory, parseToolOutput } from '../../src/renderer/src/features/chat/toolOutputParser'
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

  it('preserves XML-like metadata tags inside source file lines and strips only trailing metadata', () => {
    const source = [
      'const cwdRe = /<cwd>([\\s\\S]*?)<\\/cwd>/',
      'const diag = "<project_diagnostics>inside</project_diagnostics>"',
      'const sum = "<diagnostic_summary>inline</diagnostic_summary>"'
    ].join('\n')
    const raw = `<file>\n${source}\n</file>\n<cwd>/workspace</cwd>\n<diagnostic_summary>\nCurrent file: 0 errors\n</diagnostic_summary>`
    const parsed = parseToolOutput(raw)
    expect(parsed.cleanText).toBe(source)
    expect(parsed.cwd).toBe('/workspace')
    expect(parsed.diagnostics).toBeNull()
    expect(parsed.diagnosticSummary).toBe('Current file: 0 errors')
  })

  it('strips common prefix and suffix before LCS threshold on large diffs', () => {
    const prefix = Array.from({ length: 300 }, (_, i) => `const prefix_${i} = ${i}`)
    const suffix = Array.from({ length: 300 }, (_, i) => `const suffix_${i} = ${i}`)
    const oldText = [...prefix, 'const target = "old"', ...suffix].join('\n')
    const newText = [...prefix, 'const target = "new"', ...suffix].join('\n')
    const diff = computeEditDiff(oldText, newText)
    expect(diff.added).toBe(1)
    expect(diff.removed).toBe(1)
    expect(diff.lines.some((line) => line.kind === 'del' && line.oldLine === 301)).toBe(true)
    expect(diff.lines.some((line) => line.kind === 'add' && line.newLine === 301)).toBe(true)
  })

  it('classifies sudo, web_fetch, lsp, ssh, and custom_agent tools accurately', () => {
    expect(classifyToolCategory('sudo')).toBe('command')
    expect(classifyToolCategory('web_fetch')).toBe('search')
    expect(classifyToolCategory('lsp_call_hierarchy')).toBe('read')
    expect(classifyToolCategory('lsp_restart')).toBe('read')
    expect(classifyToolCategory('mcp_ssh_ssh_read_file')).toBe('read')
    expect(classifyToolCategory('mcp_ssh_ssh_write_file')).toBe('write')
    expect(classifyToolCategory('mcp_ssh_ssh_edit_file')).toBe('edit')
    expect(classifyToolCategory('custom_agent_dispatch')).toBe('task')
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

  it('does not inherit finish from turn 1 when turn 2 is still running', () => {
    const merged = mergeConsecutiveToolMessages([
      makeMsg('a1', [
        { type: 'tool_call', data: { id: 'c1', name: 'edit', input: '{}', result: { content: 'ok' } } },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]),
      makeMsg('a2', [
        { type: 'tool_call', data: { id: 'c2', name: 'bash', input: '{"command":"npm test"}' } }
      ])
    ])
    expect(merged).toHaveLength(1)
    expect(merged[0].parts.some((part) => part.type === 'finish')).toBe(false)

    const html = renderToStaticMarkup(
      <I18nProvider language="en">
        <ToolExecutionGroup
          messageFinished={merged[0].parts.some((part) => part.type === 'finish')}
          parts={merged[0].parts}
        />
      </I18nProvider>
    )
    expect(html).toContain('is-running')
    expect(html).not.toContain('has-error')
  })

  it('uses singular action label in English error heading for 1 item', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="en">
        <ToolExecutionGroup
          messageFinished
          parts={[
            {
              type: 'tool_call',
              data: { id: 'c1', name: 'bash', input: '{"command":"false"}', result: { content: 'fail', is_error: true } }
            }
          ]}
        />
      </I18nProvider>
    )
    expect(html).toContain('Completed with errors · 1 action')
    expect(html).not.toContain('1 actions')
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

  it('starts collapsed by default both when running and when completed', () => {
    const completedHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolExecutionGroup
          messageFinished
          parts={[
            {
              type: 'tool_call',
              data: { id: 'c1', name: 'read_file', input: '{"path":"a.ts"}', result: { content: 'ok' } }
            },
            {
              type: 'tool_call',
              data: { id: 'c2', name: 'grep', input: '{"pattern":"foo"}', result: { content: 'ok' } }
            }
          ]}
        />
      </I18nProvider>
    )
    expect(completedHtml).not.toContain('open=""')

    const runningHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolExecutionGroup
          parts={[
            {
              type: 'tool_call',
              data: { id: 'c1', name: 'read_file', input: '{"path":"a.ts"}', result: { content: 'ok' } }
            },
            {
              type: 'tool_call',
              data: { id: 'c2', name: 'bash', input: '{"command":"npm test"}' }
            }
          ]}
        />
      </I18nProvider>
    )
    expect(runningHtml).not.toContain('open=""')
  })

  it('merges work turns with reasoning and single tool calls into one group and keeps only the latest reasoning', () => {
    const merged = mergeConsecutiveToolMessages([
      makeMsg('a1', [
        { type: 'reasoning', data: { thinking: 'Step 1 thinking', started_at: 10, finished_at: 12 } },
        { type: 'tool_call', data: { id: 'c1', name: 'view', input: '{"file_path":"a.ts"}', result: { content: 'ok' } } },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]),
      makeMsg('a2', [
        { type: 'reasoning', data: { thinking: 'Step 2 thinking', started_at: 13, finished_at: 17 } },
        { type: 'tool_call', data: { id: 'c2', name: 'edit', input: '{"file_path":"a.ts"}', result: { content: 'ok' } } },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]),
      makeMsg('a3', [
        { type: 'reasoning', data: { thinking: 'Step 3 thinking', started_at: 18, finished_at: 19 } },
        { type: 'tool_call', data: { id: 'c3', name: 'mcp_browser_browser_run', input: '{"steps":[]}', result: { content: 'ok' } } },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]),
      makeMsg('a4', [
        { type: 'text', data: { text: 'All done!' } },
        { type: 'finish', data: { reason: 'end_turn' } }
      ])
    ])

    expect(merged).toHaveLength(2)
    const reasonings = merged[0].parts.filter((p) => p.type === 'reasoning')
    expect(reasonings).toHaveLength(1)
    expect(reasonings[0].data.thinking).toBe('Step 3 thinking')
    expect(merged[0].parts.filter((p) => p.type === 'tool_call')).toHaveLength(3)

    const segments = partitionMessageParts(merged[0].parts)
    const visibleSegments = segments.filter(
      (s) => s.kind === 'tool-group' || s.part.type !== 'finish'
    )
    expect(visibleSegments.map((s) => s.kind)).toEqual(['part', 'tool-group'])
    expect(visibleSegments[1].kind === 'tool-group' && visibleSegments[1].parts).toHaveLength(3)

    const singleSegments = partitionMessageParts([
      { type: 'tool_call', data: { id: 'solo', name: 'view', input: '{"file_path":"only.ts"}' } }
    ])
    expect(singleSegments).toHaveLength(1)
    expect(singleSegments[0].kind).toBe('tool-group')
  })

  it('drops work reasoning when the following assistant answer starts with a newer reasoning', () => {
    const merged = mergeConsecutiveToolMessages([
      makeMsg('a1', [
        { type: 'reasoning', data: { thinking: 'Old thinking', started_at: 10, finished_at: 12 } },
        { type: 'tool_call', data: { id: 'c1', name: 'view', input: '{}', result: { content: 'ok' } } },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]),
      makeMsg('a2', [
        { type: 'reasoning', data: { thinking: 'Final answer thinking', started_at: 13, finished_at: 14 } },
        { type: 'text', data: { text: 'Done!' } },
        { type: 'finish', data: { reason: 'end_turn' } }
      ])
    ])

    expect(merged).toHaveLength(2)
    expect(merged[0].parts.some((p) => p.type === 'reasoning')).toBe(false)
    expect(merged[1].parts.filter((p) => p.type === 'reasoning')).toHaveLength(1)
  })

  it('renders browser_run steps as a compact scenario list instead of raw JSON', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolBody
          name="mcp_browser_browser_run"
          isShell={false}
          standalone={false}
          input={{
            steps: [
              { action: 'resize', width: 1440 },
              { action: 'navigate', url: 'http://127.0.0.1:5173' },
              { action: 'click', selector: '.tool-exec-group-summary' },
              { action: 'screenshot', name: 'overview', zoom: 2 }
            ]
          }}
          description=""
          command={null}
          output="Scenario PASSED: 4 of 4 steps run."
          outputHidden={false}
          completed
          interrupted={false}
          failed={false}
        />
      </I18nProvider>
    )
    expect(html).toContain('tool-scenario-list')
    expect(html).toContain('tool-scenario-step')
    expect(html).toContain('Шагов: 4')
    expect(html).toContain('1440px')
    expect(html).toContain('http://127.0.0.1:5173')
    expect(html).not.toContain('tool-kv-list')
  })
})

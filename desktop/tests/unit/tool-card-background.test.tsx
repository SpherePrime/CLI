import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { backgroundJob } from '../../src/renderer/src/features/chat/backgroundJob'
import { ToolCard } from '../../src/renderer/src/features/chat/ToolCard'
import type { Message } from '../../src/shared/contracts'

function makeToolPart(metadata: unknown): Message['parts'][number] {
  return {
    type: 'tool_call',
    data: {
      id: 'call-1',
      name: 'bash',
      input: JSON.stringify({ command: 'npm run dev' }),
      result: {
        content: 'started in background',
        is_error: false,
        metadata
      }
    }
  }
}

describe('backgroundJob and ToolCard background badge', () => {
  it('accepts job_id and shell_id when background is strictly true', () => {
    expect(backgroundJob(JSON.stringify({ background: true, job_id: 'job-12' }))).toBe('job-12')
    expect(backgroundJob(JSON.stringify({ background: true, shell_id: 'shell-34' }))).toBe(
      'shell-34'
    )
    expect(
      backgroundJob(JSON.stringify({ background: true, job_id: '', shell_id: 'shell-35' }))
    ).toBe('shell-35')
    expect(backgroundJob({ background: true, shell_id: 'shell-obj' })).toBe('shell-obj')
  })

  it('ignores non-background, missing ids and malformed metadata', () => {
    expect(backgroundJob(JSON.stringify({ background: false, shell_id: 'shell-1' }))).toBe('')
    expect(backgroundJob(JSON.stringify({ background: 'true', shell_id: 'shell-1' }))).toBe('')
    expect(backgroundJob(JSON.stringify({ shell_id: 'shell-1' }))).toBe('')
    expect(backgroundJob(JSON.stringify({ background: true }))).toBe('')
    expect(backgroundJob('{bad json')).toBe('')
    expect(backgroundJob('')).toBe('')
    expect(backgroundJob(undefined)).toBe('')
    expect(backgroundJob(null)).toBe('')
  })

  it('renders background badge for bash shell_id and job_id tool results', () => {
    const shellMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolCard part={makeToolPart(JSON.stringify({ background: true, shell_id: 'shell-42' }))} />
      </I18nProvider>
    )
    expect(shellMarkup).toContain('class="tool-background"')
    expect(shellMarkup).toContain('В фоне · shell-42')
    expect(shellMarkup).toContain('class="tool-status background"')

    const englishMarkup = renderToStaticMarkup(
      <I18nProvider language="en">
        <ToolCard part={makeToolPart(JSON.stringify({ background: true, shell_id: 'shell-42' }))} />
      </I18nProvider>
    )
    expect(englishMarkup).toContain('Background · shell-42')

    const jobMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolCard part={makeToolPart(JSON.stringify({ background: true, job_id: 'job-99' }))} />
      </I18nProvider>
    )
    expect(jobMarkup).toContain('В фоне · job-99')

    const completedMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolCard part={makeToolPart(JSON.stringify({ background: false, shell_id: 'shell-42' }))} />
      </I18nProvider>
    )
    expect(completedMarkup).not.toContain('class="tool-background"')
    expect(completedMarkup).toContain('class="tool-status done"')
  })
})

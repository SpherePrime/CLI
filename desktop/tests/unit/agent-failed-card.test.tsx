import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { consolidateToolMessages } from '../../src/shared/toolMessages'
import type { Message } from '../../src/shared/contracts'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { AgentCard } from '../../src/renderer/src/features/chat/AgentCard'

const message = (id: string, role: string, parts: any[]) => ({ id, role, parts }) as unknown as Message

describe('failed agent launch hiding', () => {
  it('omits rejected agent tool_call and tool_result when is_error has no metadata job_id', () => {
    const shown = consolidateToolMessages([
      message('u1', 'user', [{ type: 'text', data: { text: 'Start workers' } }]),
      message('a1', 'assistant', [
        { type: 'tool_call', data: { id: 'c1', name: 'agent', input: '{"name":"Worker","prompt":"x"}' } }
      ]),
      message('t1', 'tool', [
        {
          type: 'tool_result',
          data: { tool_call_id: 'c1', name: 'agent', content: 'Rejected', is_error: true }
        }
      ]),
      message('a2', 'assistant', [{ type: 'text', data: { text: 'Retried cleanly' } }])
    ])
    expect(shown.map((item) => item.id)).toEqual(['u1', 'a2'])
  })

  it('omits only rejected agent call while keeping other parts in the same assistant message', () => {
    const shown = consolidateToolMessages([
      message('u1', 'user', [{ type: 'text', data: { text: 'Start' } }]),
      message('a1', 'assistant', [
        { type: 'text', data: { text: 'Launching...' } },
        { type: 'tool_call', data: { id: 'c1', name: 'agent', input: '{"name":"Bad","prompt":"x"}' } },
        { type: 'tool_call', data: { id: 'c2', name: 'agent', input: '{"name":"Good","prompt":"y"}' } }
      ]),
      message('t1', 'tool', [
        {
          type: 'tool_result',
          data: { tool_call_id: 'c1', name: 'agent', content: 'Rejected', is_error: true, metadata: '{}' }
        },
        {
          type: 'tool_result',
          data: { tool_call_id: 'c2', name: 'agent', content: 'Done', is_error: false }
        }
      ])
    ])
    expect(shown.map((item) => item.id)).toEqual(['u1', 'a1'])
    expect(shown[1].parts.map((p) => p.type)).toEqual(['text', 'tool_call'])
    expect(shown[1].parts[1].data.id).toBe('c2')
  })

  it('preserves running, succeeded, and launched errored agent calls', () => {
    const shown = consolidateToolMessages([
      message('u1', 'user', [{ type: 'text', data: { text: 'Run' } }]),
      message('a1', 'assistant', [
        { type: 'tool_call', data: { id: 'c-running', name: 'agent', input: '{"name":"Running"}' } },
        { type: 'tool_call', data: { id: 'c-done', name: 'agent', input: '{"name":"Done"}' } },
        { type: 'tool_call', data: { id: 'c-err-obj', name: 'agent', input: '{"name":"ErrObj"}' } },
        { type: 'tool_call', data: { id: 'c-err-str', name: 'agent', input: '{"name":"ErrStr"}' } }
      ]),
      message('t1', 'tool', [
        {
          type: 'tool_result',
          data: { tool_call_id: 'c-done', name: 'agent', content: 'ok', is_error: false }
        },
        {
          type: 'tool_result',
          data: {
            tool_call_id: 'c-err-obj',
            name: 'agent',
            content: 'worker crashed',
            is_error: true,
            metadata: { job_id: 'job-1' }
          }
        },
        {
          type: 'tool_result',
          data: {
            tool_call_id: 'c-err-str',
            name: 'agent',
            content: 'worker crashed',
            is_error: true,
            metadata: JSON.stringify({ background: false, job_id: 'job-2' })
          }
        }
      ])
    ])
    expect(shown.map((item) => item.id)).toEqual(['u1', 'a1'])
    expect(shown[1].parts.map((p) => p.data.id)).toEqual([
      'c-running',
      'c-done',
      'c-err-obj',
      'c-err-str'
    ])
    expect(shown[1].parts[0].data.result).toBeUndefined()
  })

  it('AgentCard returns null when result.is_error is true without metadata.job_id and renders when job_id exists', () => {
    const unlaunchedMarkup = renderToStaticMarkup(
      <I18nProvider language="en">
        <AgentCard
          messageId="m1"
          part={{
            type: 'tool_call',
            data: {
              id: 'c1',
              name: 'agent',
              input: JSON.stringify({ name: 'Worker', prompt: 'Work' }),
              result: { is_error: true, content: 'Rejected' }
            }
          }}
        />
      </I18nProvider>
    )
    expect(unlaunchedMarkup).toBe('')

    const launchedMarkup = renderToStaticMarkup(
      <I18nProvider language="en">
        <AgentCard
          messageId="m1"
          part={{
            type: 'tool_call',
            data: {
              id: 'c2',
              name: 'agent',
              input: JSON.stringify({ name: 'Worker', prompt: 'Work' }),
              result: {
                is_error: true,
                content: 'Failed',
                metadata: JSON.stringify({ background: false, job_id: 'job-42' })
              }
            }
          }}
        />
      </I18nProvider>
    )
    expect(launchedMarkup).toContain('agent-card tool-error')
  })
})

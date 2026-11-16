import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { AgentsPanel } from '../../src/renderer/src/features/agents/AgentsPanel'
import {
  agentRosterPollInterval,
  agentWorkerId,
  consumeAgentRequestState,
  scopeAgentRequest
} from '../../src/renderer/src/features/agents/openAgent'
import type { AgentWorker } from '../../src/shared/agents'
import type { Workspace } from '../../src/renderer/src/app/useWorkspace'

function makeWorker(overrides: Partial<AgentWorker> = {}): AgentWorker {
  return {
    id: 'msg-1$$call-1',
    session_id: 'worker-session-1',
    parent_session_id: 'session-1',
    root_session_id: 'session-1',
    name: 'Reviewer',
    agent: 'code',
    task: 'Check changes',
    status: 'done',
    started_at: 1,
    updated_at: 2,
    ...overrides
  }
}

function makeWorkspace(sessionId = 'session-1'): Workspace {
  return {
    project: { id: 'project-1', path: '/repo', name: 'Repo', kind: 'project' },
    session: { id: sessionId, title: 'Main chat' },
    busy: false,
    config: {},
    preferences: {
      theme: 'dark',
      language: 'ru',
      recent: [],
      pinned: [],
      drafts: {},
      sidebarWidth: 256,
      inspectorWidth: 334
    },
    call: async () => [],
    send: async () => true,
    errorHandler: () => {}
  } as unknown as Workspace
}

describe('agent open request and AgentsPanel selection', () => {
  it('builds worker ids and scopes requests to the active session', () => {
    expect(agentWorkerId('msg-1', 'call-1')).toBe('msg-1$$call-1')
    const req = { workerId: 'msg-1$$call-1', sequence: 4, sessionId: 'session-1' }
    expect(scopeAgentRequest(req, 'session-1')).toEqual(req)
    expect(scopeAgentRequest(req, 'session-2')).toBeUndefined()
    expect(scopeAgentRequest({ workerId: 'msg-1$$call-1', sequence: 5 }, 'session-2')).toEqual({
      workerId: 'msg-1$$call-1',
      sequence: 5
    })
    expect(scopeAgentRequest(undefined, 'session-1')).toBeUndefined()
  })

  it('consumes only matching request sequence without clearing newer requests', () => {
    const first = { workerId: 'w1', sequence: 1, sessionId: 's1' }
    const second = { workerId: 'w2', sequence: 2, sessionId: 's1' }
    expect(consumeAgentRequestState(first, 1)).toBeUndefined()
    expect(consumeAgentRequestState(second, 1)).toEqual(second)
    expect(consumeAgentRequestState(undefined, 1)).toBeUndefined()
  })

  it('polls at 400ms while a selected worker is not yet in the roster', () => {
    const idleWorker = makeWorker({ id: 'w1', status: 'done' })
    const activeWorker = makeWorker({ id: 'w2', status: 'running' })

    expect(agentRosterPollInterval([], 'w1')).toBe(400)
    expect(agentRosterPollInterval([idleWorker], 'w2')).toBe(400)
    expect(agentRosterPollInterval([idleWorker], 'w1')).toBe(3000)
    expect(agentRosterPollInterval([idleWorker, activeWorker], 'w1')).toBe(1000)
    expect(agentRosterPollInterval([], '')).toBe(3000)
    expect(agentRosterPollInterval([activeWorker], '')).toBe(1000)
  })

  it('renders pending opening state instead of empty roster when worker is not registered yet', () => {
    const markupRu = renderToStaticMarkup(
      <I18nProvider language="ru">
        <AgentsPanel
          workspace={makeWorkspace('session-1')}
          request={{ workerId: 'msg-1$$call-1', sequence: 7, sessionId: 'session-1' }}
        />
      </I18nProvider>
    )
    expect(markupRu).toContain('Запуск агента…')
    expect(markupRu).toContain('aria-label="К списку агентов"')
    expect(markupRu).not.toContain('В этом чате пока нет агентов')

    const markupEn = renderToStaticMarkup(
      <I18nProvider language="en">
        <AgentsPanel
          workspace={makeWorkspace('session-1')}
          request={{ workerId: 'msg-1$$call-1', sequence: 7, sessionId: 'session-1' }}
        />
      </I18nProvider>
    )
    expect(markupEn).toContain('Starting agent…')
    expect(markupEn).toContain('aria-label="Back to agents"')
  })

  it('ignores open requests belonging to another session', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <AgentsPanel
          workspace={makeWorkspace('session-2')}
          request={{ workerId: 'msg-1$$call-1', sequence: 8, sessionId: 'session-1' }}
        />
      </I18nProvider>
    )
    expect(markup).not.toContain('Запуск агента…')
    expect(markup).toContain('Загрузка…')
  })
})

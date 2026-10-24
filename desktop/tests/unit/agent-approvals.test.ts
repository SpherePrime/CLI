import { describe, expect, it } from 'vitest'
import { agentApprovalSessions } from '../../src/renderer/src/features/agents/agentApprovalSessions'
import type { AgentWorker } from '../../src/shared/agents'

describe('agent approval ownership', () => {
  it('includes primary and verified nested team sessions but excludes unrelated roots', () => {
    const workers = [
      { session_id: 'worker', root_session_id: 'root', name: 'Builder' },
      { session_id: 'nested', root_session_id: 'root', name: 'Researcher' },
      { session_id: 'foreign', root_session_id: 'other', name: 'Other' }
    ] as AgentWorker[]
    const sessions = agentApprovalSessions('root', workers)
    expect([...sessions.keys()]).toEqual(['root', 'worker', 'nested'])
    expect(sessions.get('worker')).toBe('Builder')
    expect(agentApprovalSessions('new-root', workers).has('worker')).toBe(false)
  })
})

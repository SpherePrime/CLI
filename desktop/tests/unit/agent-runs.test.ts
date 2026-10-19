import { expect, it, vi } from 'vitest'
import type { Project } from '../../src/shared/contracts'
import type { AutomationRun } from '../../src/shared/automations'
import { createAgentAdapter } from '../../src/main/workbench/agentRuns'

function fixture() {
  const project: Project = {
    id: 'current-workspace',
    path: '/project',
    clientId: 'client',
    connection: 'ready'
  }
  const call = vi.fn(async (_id: string, operation: string) =>
    operation === 'createSession' ? { id: 'automation-session' } : { status: 'completed' }
  )
  const open = vi.fn(async () => project)
  const service = { projects: new Map([[project.id, project]]), open, call }
  const adapter = createAgentAdapter({ service: () => service })
  const run: AutomationRun = {
    id: 'stable-run',
    ruleId: 'rule',
    root: '/project',
    title: 'Checks',
    trigger: 'manual',
    status: 'running',
    createdAt: 1,
    workspaceId: 'stale-workspace',
    sessionId: 'automation-session'
  }
  return { service, adapter, call, open, run, project }
}

it('creates an independent session and submits the persisted run id without changing the selected chat', async () => {
  const { adapter, call } = fixture()
  const prepared = await adapter.prepare({ root: '/project', title: 'Checks', runId: 'stable-run' })
  await adapter.dispatch({
    root: '/project',
    ...prepared,
    runId: 'stable-run',
    prompt: 'Run the checks'
  })
  expect(call.mock.calls).toEqual([
    ['current-workspace', 'init'],
    ['current-workspace', 'createSession', { title: 'Checks' }],
    [
      'current-workspace',
      'send',
      { sessionId: 'automation-session', runId: 'stable-run', prompt: 'Run the checks' }
    ]
  ])
})

it('rebinds recovery and cancellation by project root instead of a stale or foreign workspace id', async () => {
  const { adapter, call, run } = fixture()
  expect(await adapter.status(run)).toEqual({ status: 'completed' })
  await adapter.cancel(run)
  expect(call.mock.calls).toEqual([
    ['current-workspace', 'runs', { runId: run.id }],
    ['current-workspace', 'cancel', { sessionId: run.sessionId }],
    ['current-workspace', 'runs', { runId: run.id }]
  ])
})

it('distinguishes a missing persisted run from an unavailable backend', async () => {
  const { adapter, call, run } = fixture()
  call.mockRejectedValueOnce(Object.assign(new Error('missing'), { status: 404 }))
  expect((await adapter.status(run)).status).toBe('interrupted')
  call.mockRejectedValueOnce(Object.assign(new Error('offline'), { status: 503 }))
  await expect(adapter.status(run)).rejects.toThrow('offline')
})

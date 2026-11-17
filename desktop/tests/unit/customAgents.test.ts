import { describe, expect, it, vi } from 'vitest'
import { installPreviewHost } from '../../src/renderer/src/host/installPreviewHost'
import {
  createNativeBridge,
  type NativeInvoke,
  type NativeListen
} from '../../src/renderer/src/host/nativeBridge'
import {
  agentRoleLabel,
  agentStatusMeta,
  buildAgentDraft,
  filterCustomAgents,
  formatDuration,
  isAgentDraftDirty,
  modelSummary,
  runStatusMeta,
  triggerLabel
} from '../../src/renderer/src/features/customAgents/presentation'
import type { CustomAgent, ModelOption } from '../../src/shared/customAgents'
import type { DesktopEvent } from '../../src/shared/contracts'

function setupPreview() {
  const previousWindow = (globalThis as { window?: unknown }).window
  ;(globalThis as { window?: unknown }).window = { location: { search: '?auth=signed-in' } }
  installPreviewHost()
  const prime = (globalThis as { window: { prime: Window['prime'] } }).window.prime
  return {
    prime,
    restore() {
      if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window
      else (globalThis as { window?: unknown }).window = previousWindow
    }
  }
}

describe('customAgents preview host', () => {
  it('supports full CRUD lifecycle and emits events', async () => {
    vi.useFakeTimers()
    const env = setupPreview()
    try {
      const received: DesktopEvent[] = []
      const unsub = env.prime.onEvent((event) => {
        if ('kind' in event && event.kind === 'customAgents') received.push(event)
      })

      const listed = await env.prime.customAgents({ action: 'list' })
      expect(listed.agents.length).toBeGreaterThanOrEqual(3)
      expect(listed.agents.map((a: CustomAgent) => a.name)).toContain('Главный координатор')

      const created = await env.prime.customAgents({
        action: 'create',
        input: {
          name: 'Агент проверки API',
          role: 'worker',
          trigger_type: 'manual',
          model_mode: 'auto'
        }
      })
      expect(created.id).toBeTruthy()
      expect(created.name).toBe('Агент проверки API')
      expect(received.some((e) => 'kind' in e && e.data.type === 'agent.created')).toBe(true)

      const fetched = await env.prime.customAgents({ action: 'get', agentId: created.id })
      expect(fetched.id).toBe(created.id)

      const updated = await env.prime.customAgents({
        action: 'update',
        agentId: created.id,
        patch: { enabled: false, description: 'Обновлённое описание' }
      })
      expect(updated.enabled).toBe(false)
      expect(updated.status).toBe('paused')
      expect(updated.description).toBe('Обновлённое описание')

      const deleted = await env.prime.customAgents({ action: 'delete', agentId: created.id })
      expect(deleted.deleted).toBe(true)

      const afterDelete = await env.prime.customAgents({ action: 'list' })
      expect(afterDelete.agents.some((a: CustomAgent) => a.id === created.id)).toBe(false)
      unsub()
    } finally {
      env.restore()
      vi.useRealTimers()
    }
  })

  it('probes monitor targets, runs tasks synchronously/asynchronously, cancels runs, and manages blackboard/models', async () => {
    vi.useFakeTimers()
    const env = setupPreview()
    try {
      const events: string[] = []
      const unsub = env.prime.onEvent((event) => {
        if ('kind' in event && event.kind === 'customAgents') events.push(event.data.type)
      })

      const probe = await env.prime.customAgents({
        action: 'probe',
        agentId: 'agent-release-monitor'
      })
      expect(probe.changed).toBe(true)
      expect(probe.hash).toBeTruthy()
      expect(probe.diff).toContain('--- previous')

      const syncRun = await env.prime.customAgents({
        action: 'run',
        agentId: 'agent-release-monitor',
        prompt: 'Проверь релиз вручную',
        sync: true
      })
      expect(syncRun.status).toBe('passed')
      expect(syncRun.verifier_verdict).toBe('PASS')
      expect(syncRun.resolved_model).toBeTruthy()
      expect(events).toContain('blackboard.posted')

      const asyncRun = await env.prime.customAgents({
        action: 'run',
        agentId: 'agent-orchestrator',
        prompt: 'Подготовь план проверки'
      })
      expect(asyncRun.status).toBe('running')

      const cancelled = await env.prime.customAgents({
        action: 'cancelRun',
        agentId: 'agent-orchestrator',
        runId: asyncRun.id
      })
      expect(cancelled.cancelled).toBe(true)

      const runs = await env.prime.customAgents({
        action: 'runs',
        agentId: 'agent-orchestrator',
        limit: 5
      })
      expect(runs.runs[0].status).toBe('cancelled')

      const posted = await env.prime.customAgents({
        action: 'postBlackboard',
        input: {
          chain_id: 'chain-test-1',
          topic: 'handoff',
          payload: 'Передача тестового контекста'
        }
      })
      expect(posted.chain_id).toBe('chain-test-1')

      const board = await env.prime.customAgents({
        action: 'blackboard',
        chainId: 'chain-test-1'
      })
      expect(board.entries.length).toBe(1)
      expect(board.entries[0].payload).toBe('Передача тестового контекста')

      const modelsRes = await env.prime.customAgents({ action: 'models' })
      expect(modelsRes.models.length).toBeGreaterThan(0)
      unsub()
    } finally {
      env.restore()
      vi.useRealTimers()
    }
  })
})

describe('customAgents presentation helpers and nativeBridge normalization', () => {
  it('filters agents by query and role', async () => {
    vi.useFakeTimers()
    const env = setupPreview()
    try {
      const { agents } = await env.prime.customAgents({ action: 'list' })
      expect(filterCustomAgents(agents, '', 'orchestrator').length).toBe(1)
      expect(filterCustomAgents(agents, 'релиз', 'all').map((a) => a.slug)).toEqual([
        'release-monitor'
      ])
      expect(filterCustomAgents(agents, 'missing', 'all')).toEqual([])
    } finally {
      env.restore()
      vi.useRealTimers()
    }
  })

  it('formats roles, statuses, triggers, models, durations and draft dirtiness', () => {
    expect(agentRoleLabel('orchestrator')).toBe('Координатор')
    expect(agentRoleLabel('verifier')).toBe('Верификатор')
    expect(agentRoleLabel('worker')).toBe('Воркер')

    expect(agentStatusMeta('monitoring')).toEqual({ label: 'Мониторит', tone: 'accent' })
    expect(agentStatusMeta('rework')).toEqual({ label: 'Доработка', tone: 'amber' })
    expect(runStatusMeta('skipped_no_change')).toEqual({ label: 'Без изменений', tone: 'neutral' })
    expect(runStatusMeta('passed')).toEqual({ label: 'Пройден', tone: 'green' })

    expect(
      triggerLabel({
        trigger_type: 'monitor',
        interval_minutes: 15,
        cron_expr: '',
        monitor_kind: 'http'
      })
    ).toBe('Монитор (HTTP) · 15 мин')
    expect(
      triggerLabel({
        trigger_type: 'cron',
        interval_minutes: 0,
        cron_expr: '0 * * * *',
        monitor_kind: ''
      })
    ).toBe('Cron: 0 * * * *')

    const models: ModelOption[] = [
      {
        provider_id: 'openai',
        provider_name: 'OpenAI',
        model_id: 'gpt-5.4',
        name: 'GPT-5.4',
        tier: 'reasoning',
        can_reason: true,
        reasoning_modes: ['low', 'high'],
        reasoning_levels: ['low', 'high'],
        default_reasoning_mode: 'high',
        context_window: 200000,
        cost_per_1m_in: 3,
        cost_per_1m_out: 12,
        locked: false,
        plan_label: 'Pro'
      }
    ]
    expect(
      modelSummary({ model_mode: 'auto', provider_id: '', model_id: '' }, models)
    ).toBe('Авто')
    expect(
      modelSummary({ model_mode: 'manual', provider_id: 'openai', model_id: 'gpt-5.4' }, models)
    ).toBe('GPT-5.4')

    expect(formatDuration(100, 145)).toBe('45 с')
    expect(formatDuration(100, 225)).toBe('2 м 5 с')

    const sample: CustomAgent = {
      id: 'a1',
      slug: 'a1',
      name: 'Test',
      description: '',
      role: 'worker',
      system_prompt: 'Prompt',
      memory: '',
      model_mode: 'auto',
      provider_id: '',
      model_id: '',
      reasoning_mode: '',
      auto_model_hint: '',
      allowed_tools: [],
      workspace_path: '',
      enabled: true,
      created_by: 'user',
      trigger_type: 'manual',
      interval_minutes: 0,
      cron_expr: '',
      monitor_kind: '',
      monitor_target: '',
      monitor_last_hash: '',
      monitor_last_snapshot: '',
      acceptance_criteria: '',
      deterministic_checks: [],
      verifier_agent_id: '',
      next_agent_id: '',
      max_rework_attempts: 3,
      status: 'idle',
      last_error: '',
      last_run_at: 0,
      next_run_at: 0,
      created_at: 1,
      updated_at: 1
    }
    const draft = buildAgentDraft(sample)
    expect(isAgentDraftDirty(sample, draft)).toBe(false)
    expect(isAgentDraftDirty(sample, { ...draft, name: 'Updated' })).toBe(true)
  })

  it('forwards customAgents calls and normalizes native customAgents events', async () => {
    let receive: Parameters<NativeListen>[1]
    const invoke = vi.fn().mockResolvedValue({ agents: [] })
    const bridge = await createNativeBridge(
      invoke as NativeInvoke,
      async (_eventName, listener) => {
        receive = listener
        return vi.fn<() => void>()
      }
    )

    await bridge.api.customAgents({ action: 'list' })
    expect(invoke).toHaveBeenCalledWith('desktop_call', {
      method: 'customAgents',
      args: [{ action: 'list' }]
    })

    const received: DesktopEvent[] = []
    bridge.api.onEvent((event) => received.push(event))
    receive!({
      payload: {
        workspaceId: '',
        type: 'customAgents',
        payload: {
          type: 'run.started',
          agent_id: 'a1',
          run_id: 'r1',
          chain_id: 'c1',
          payload: {},
          timestamp: 100
        }
      } as unknown as DesktopEvent
    })

    expect(received).toHaveLength(1)
    expect(received[0]).toEqual({
      workspaceId: '',
      kind: 'customAgents',
      data: {
        type: 'run.started',
        agent_id: 'a1',
        run_id: 'r1',
        chain_id: 'c1',
        payload: {},
        timestamp: 100
      }
    })
    bridge.dispose()
  })
})

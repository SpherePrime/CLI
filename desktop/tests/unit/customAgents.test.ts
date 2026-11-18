import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { installPreviewHost } from '../../src/renderer/src/host/installPreviewHost'
import {
  createNativeBridge,
  type NativeInvoke,
  type NativeListen
} from '../../src/renderer/src/host/nativeBridge'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { AgentInspector } from '../../src/renderer/src/features/customAgents/AgentInspector'
import { AgentWorkspace } from '../../src/renderer/src/features/customAgents/AgentWorkspace'
import { WorkflowCanvas } from '../../src/renderer/src/features/customAgents/WorkflowCanvas'
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
import {
  buildBezierPath,
  canConnectAgents,
  computeAutoLayout,
  computeFitViewport,
  getPortPoint,
  loadSavedLayout,
  mergeNodePositions,
  PORT_IN_OFFSET_Y,
  PORT_NEXT_OFFSET_Y,
  PORT_VERIFIER_OFFSET_Y,
  saveLayout,
  WORKFLOW_NODE_HEIGHT,
  WORKFLOW_NODE_WIDTH
} from '../../src/renderer/src/features/customAgents/workflowLayout'
import type {
  CustomAgent,
  GlobalMemoryDoc,
  ModelOption,
  SwarmResult
} from '../../src/shared/customAgents'
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
      expect(listed.agents.length).toBeGreaterThanOrEqual(4)
      expect(listed.agents.map((a: CustomAgent) => a.name)).toContain('Главный координатор')
      expect(listed.agents.map((a: CustomAgent) => a.name)).toContain('Аналитик изменений')

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

  it('loads and saves global memory while enforcing character limits', async () => {
    vi.useFakeTimers()
    const env = setupPreview()
    try {
      const initial = (await env.prime.customAgents({
        action: 'getGlobalMemory'
      })) as GlobalMemoryDoc
      expect(initial.memory_limit).toBe(2200)
      expect(initial.user_limit).toBe(1375)
      expect(initial.memory_chars).toBe(Array.from(initial.memory).length)
      expect(initial.user_chars).toBe(Array.from(initial.user_profile).length)

      const saved = (await env.prime.customAgents({
        action: 'saveGlobalMemory',
        input: {
          memory: '- Новая заметка среды',
          user_profile: '- Краткий стиль ответов',
          soul: 'Чёткий рабочий тон'
        }
      })) as GlobalMemoryDoc
      expect(saved.memory).toBe('- Новая заметка среды')
      expect(saved.user_profile).toBe('- Краткий стиль ответов')
      expect(saved.soul).toBe('Чёткий рабочий тон')
      expect(saved.memory_chars).toBe(Array.from('- Новая заметка среды').length)
      expect(saved.user_chars).toBe(Array.from('- Краткий стиль ответов').length)

      await expect(
        env.prime.customAgents({
          action: 'saveGlobalMemory',
          input: {
            memory: 'x'.repeat(2201),
            user_profile: saved.user_profile,
            soul: saved.soul
          }
        })
      ).rejects.toThrow(/2200/)

      await expect(
        env.prime.customAgents({
          action: 'saveGlobalMemory',
          input: {
            memory: saved.memory,
            user_profile: 'u'.repeat(1376),
            soul: saved.soul
          }
        })
      ).rejects.toThrow(/1375/)

      const afterFailed = (await env.prime.customAgents({
        action: 'getGlobalMemory'
      })) as GlobalMemoryDoc
      expect(afterFailed.memory).toBe('- Новая заметка среды')
      expect(afterFailed.user_profile).toBe('- Краткий стиль ответов')
    } finally {
      env.restore()
      vi.useRealTimers()
    }
  })

  it('dispatches parallel swarm, updates blackboard and runs, and supports strict_verify', async () => {
    vi.useFakeTimers()
    const env = setupPreview()
    try {
      const events: string[] = []
      const unsub = env.prime.onEvent((event) => {
        if ('kind' in event && event.kind === 'customAgents') events.push(event.data.type)
      })

      const strictWorker = (await env.prime.customAgents({
        action: 'create',
        input: {
          name: 'Строгий исполнитель',
          role: 'worker',
          strict_verify: true
        }
      })) as CustomAgent
      expect(strictWorker.strict_verify).toBe(true)

      const strictRun = await env.prime.customAgents({
        action: 'run',
        agentId: strictWorker.id,
        prompt: 'Проверь конфигурацию',
        sync: true
      })
      expect(strictRun.verifier_verdict).toBe('PASS')

      await expect(
        env.prime.customAgents({
          action: 'dispatchSwarm',
          input: { goal: '', workers: [{ agent_id: strictWorker.id }] }
        })
      ).rejects.toThrow()

      const swarm = (await env.prime.customAgents({
        action: 'dispatchSwarm',
        input: {
          goal: 'Провести параллельный аудит релиза',
          acceptance_criteria: 'Все подзадачи закрыты с подтверждением',
          workers: [
            { agent_id: 'agent-release-monitor', task: 'Проверить тег релиза' },
            { agent_id: strictWorker.id, task: 'Проверить конфигурацию сборки' }
          ]
        }
      })) as SwarmResult

      expect(swarm.status).toBe('passed')
      expect(swarm.verifier_verdict).toBe('PASS')
      expect(swarm.workers).toHaveLength(2)
      expect(swarm.workers.every((worker) => worker.status === 'passed')).toBe(true)
      expect(swarm.final_output).toContain('Провести параллельный аудит релиза')
      expect(events).toContain('swarm.started')
      expect(events).toContain('swarm.completed')
      expect(events).toContain('blackboard.posted')

      const board = await env.prime.customAgents({
        action: 'blackboard',
        chainId: swarm.chain_id
      })
      const topics = board.entries.map((entry: { topic: string }) => entry.topic)
      expect(topics).toContain('swarm.goal')
      expect(topics).toContain('swarm.worker')
      expect(topics).toContain('swarm.verdict')
      expect(topics).toContain('swarm.final')
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
      strict_verify: false,
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
    expect(isAgentDraftDirty(sample, { ...draft, strict_verify: true })).toBe(true)
    expect(isAgentDraftDirty(sample, { ...draft, enabled: false })).toBe(true)
  })

  it('computes topological DAG auto-layout, prevents cycles/self-links, and persists layout', () => {
    const baseAgent = (overrides: Partial<CustomAgent>): CustomAgent => ({
      id: 'a',
      slug: 'a',
      name: 'Agent',
      description: '',
      role: 'worker',
      system_prompt: '',
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
      strict_verify: false,
      status: 'idle',
      last_error: '',
      last_run_at: 0,
      next_run_at: 0,
      created_at: 1,
      updated_at: 1,
      ...overrides
    })

    const n1 = baseAgent({
      id: 'n1',
      name: 'Монитор',
      next_agent_id: 'n2',
      verifier_agent_id: 'v1'
    })
    const n2 = baseAgent({
      id: 'n2',
      name: 'Аналитик',
      next_agent_id: 'n3',
      verifier_agent_id: 'v1'
    })
    const n3 = baseAgent({
      id: 'n3',
      name: 'Координатор',
      role: 'orchestrator',
      verifier_agent_id: 'v1'
    })
    const v1 = baseAgent({
      id: 'v1',
      name: 'Верификатор',
      role: 'verifier'
    })
    const chain = [n1, n2, n3, v1]

    const layout = computeAutoLayout(chain)
    expect(layout.n1.x).toBeLessThan(layout.n2.x)
    expect(layout.n2.x).toBeLessThan(layout.n3.x)
    expect(layout.n3.x).toBeLessThan(layout.v1.x)

    const nodes = Object.values(layout)
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const overlapX = Math.abs(nodes[i].x - nodes[j].x) < WORKFLOW_NODE_WIDTH
        const overlapY = Math.abs(nodes[i].y - nodes[j].y) < WORKFLOW_NODE_HEIGHT
        expect(overlapX && overlapY).toBe(false)
      }
    }

    const merged = mergeNodePositions(chain, { n1: { x: 100, y: 120 } })
    expect(merged.n1).toEqual({ x: 100, y: 120 })
    expect(merged.n2).toBeDefined()
    expect(merged.v1).toBeDefined()

    const memStore = new Map<string, string>()
    const mockStorage = {
      getItem: (key: string) => memStore.get(key) ?? null,
      setItem: (key: string, value: string) => {
        memStore.set(key, value)
      }
    }
    saveLayout({ n1: { x: 200, y: 80 } }, mockStorage)
    expect(loadSavedLayout(mockStorage)).toEqual({ n1: { x: 200, y: 80 } })

    expect(canConnectAgents(chain, 'n1', 'n1', 'next').ok).toBe(false)
    expect(canConnectAgents(chain, 'n3', 'n1', 'next').ok).toBe(false)
    expect(canConnectAgents(chain, 'n2', 'n1', 'next').ok).toBe(false)
    expect(canConnectAgents(chain, 'n1', 'n3', 'next').ok).toBe(true)
    expect(canConnectAgents(chain, 'n1', 'v1', 'verifier').ok).toBe(true)

    expect(WORKFLOW_NODE_WIDTH).toBe(256)
    expect(WORKFLOW_NODE_HEIGHT).toBe(176)
    expect(PORT_IN_OFFSET_Y).toBe(88)
    expect(PORT_NEXT_OFFSET_Y).toBe(132)
    expect(PORT_VERIFIER_OFFSET_Y).toBe(160)

    const p1 = getPortPoint({ x: 50, y: 40 }, 'next')
    const p2 = getPortPoint({ x: 418, y: 40 }, 'in')
    expect(p1).toEqual({ x: 50 + 256, y: 40 + 132 })
    expect(p2).toEqual({ x: 418, y: 40 + 88 })

    const adjacentCurve = buildBezierPath(p1.x, p1.y, p2.x, p2.y)
    expect(adjacentCurve.path.startsWith('M ')).toBe(true)
    expect(adjacentCurve.midX).toBe((p1.x + p2.x) / 2)
    expect(adjacentCurve.midY).toBe((p1.y + p2.y) / 2)

    const longCurve = buildBezierPath(304, 200, 1152, 180, { laneOffset: 22 })
    expect(longCurve.midY).toBeGreaterThan(200)

    const fitted = computeFitViewport(layout, 960, 600)
    expect(fitted.zoom).toBeGreaterThanOrEqual(0.45)
    expect(fitted.zoom).toBeLessThanOrEqual(1)
  })

  it('renders WorkflowCanvas 3-zone nodes, port anchors, AgentWorkspace modes, and 4-tab AgentInspector', async () => {
    vi.useFakeTimers()
    const env = setupPreview()
    try {
      const { agents } = await env.prime.customAgents({ action: 'list' })
      const { models } = await env.prime.customAgents({ action: 'models' })

      const canvasHtml = renderToStaticMarkup(
        createElement(
          I18nProvider,
          { language: 'ru', children: null },
          createElement(WorkflowCanvas, {
            agents,
            models,
            selectedAgentId: agents[0].id,
            onSelectAgent: () => {},
            onConfigureAgent: () => {},
            onRunAgent: () => {},
            onUpdateAgent: async (_id, _patch) => agents[0],
            onOpenCreate: () => {},
            onOpenSwarm: () => {}
          })
        )
      )
      expect(canvasHtml).toContain('Новый узел')
      expect(canvasHtml).toContain('Запустить рой (Swarm)')
      expect(canvasHtml).toContain('Авто-раскладка')
      expect(canvasHtml).toContain('Монитор релизов')
      expect(canvasHtml).toContain('Аналитик изменений')
      expect(canvasHtml).toContain('ca-wf-port-anchor anchor-in')
      expect(canvasHtml).toContain('ca-wf-port-anchor anchor-next')
      expect(canvasHtml).toContain('ca-wf-port-anchor anchor-verifier')
      expect(canvasHtml).toContain('ca-wf-node-footer')
      expect(canvasHtml).toContain('ca-wf-selected-pill')
      expect(canvasHtml).toContain('ca-wf-edge kind-next')
      expect(canvasHtml).toContain('ca-wf-edge kind-verifier')

      const chainWorkspaceHtml = renderToStaticMarkup(
        createElement(
          I18nProvider,
          { language: 'ru', children: null },
          createElement(AgentWorkspace, {
            agent: agents[0],
            agents,
            models,
            runs: [],
            allRuns: [],
            blackboard: [],
            selectedChainId: '',
            tab: 'chain',
            onTabChange: () => {},
            onSelectChainId: () => {},
            onSelectAgent: () => {},
            onConfigureAgent: () => {},
            onRun: async () => ({} as never),
            onCancelRun: async () => ({}),
            onProbe: async () => ({
              changed: false,
              hash: '',
              snapshot: '',
              diff: '',
              error: ''
            }),
            onPostBlackboard: async () => ({} as never),
            onUpdateAgent: async () => agents[0],
            onOpenCreate: () => {},
            onOpenSwarm: () => {}
          })
        )
      )
      expect(chainWorkspaceHtml).not.toContain('ca-workspace-head')
      expect(chainWorkspaceHtml).toContain('ca-wf-viewport')

      const tasksWorkspaceHtml = renderToStaticMarkup(
        createElement(
          I18nProvider,
          { language: 'ru', children: null },
          createElement(AgentWorkspace, {
            agent: agents[0],
            agents,
            models,
            runs: [],
            allRuns: [],
            blackboard: [],
            selectedChainId: '',
            tab: 'tasks',
            onTabChange: () => {},
            onSelectChainId: () => {},
            onSelectAgent: () => {},
            onConfigureAgent: () => {},
            onRun: async () => ({} as never),
            onCancelRun: async () => ({}),
            onProbe: async () => ({
              changed: false,
              hash: '',
              snapshot: '',
              diff: '',
              error: ''
            }),
            onPostBlackboard: async () => ({} as never),
            onUpdateAgent: async () => agents[0],
            onOpenCreate: () => {},
            onOpenSwarm: () => {}
          })
        )
      )
      expect(tasksWorkspaceHtml).toContain('ca-workspace-head')
      expect(tasksWorkspaceHtml).toContain('ca-workspace-col')
      expect(tasksWorkspaceHtml).toContain('Настроить')

      const inspectorHtml = renderToStaticMarkup(
        createElement(
          I18nProvider,
          { language: 'ru', children: null },
          createElement(AgentInspector, {
            agent: agents[0],
            agents,
            models,
            projects: [],
            latestRun: null,
            busy: false,
            onSave: async () => agents[0],
            onDelete: async () => ({ deleted: true }),
            onProbe: async () => ({
              changed: false,
              hash: '',
              snapshot: '',
              diff: '',
              error: ''
            }),
            onClose: () => {}
          })
        )
      )
      expect(inspectorHtml).toContain('Профиль')
      expect(inspectorHtml).toContain('Модель')
      expect(inspectorHtml).toContain('Триггер')
      expect(inspectorHtml).toContain('Контроль')
      expect(inspectorHtml).toContain('Скрыть параметры')
    } finally {
      env.restore()
      vi.useRealTimers()
    }
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

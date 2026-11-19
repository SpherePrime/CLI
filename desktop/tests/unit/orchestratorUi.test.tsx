import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { operationRoute } from '../../src/shared/operations'
import { translate } from '../../src/shared/localization'
import {
  compactModelLabel,
  hasExplicitRoleOverride,
  isOrchestratorActive,
  orchestratorPhaseLabel,
  resolveContextWindow,
  resolveRoleConfig,
  resolveRoleDisplay,
  resolveRoleReasoning,
  roleReasoningOptions
} from '../../src/renderer/src/features/models/orchestrator'
import { ModelControl } from '../../src/renderer/src/features/models/ModelControl'
import { ModelPickerRow, OrchestratorDefaultRow } from '../../src/renderer/src/features/models/ModelPickerRow'
import { OrchestratorSettings } from '../../src/renderer/src/features/models/OrchestratorSettings'
import { MessageItem } from '../../src/renderer/src/features/chat/MessageItem'
import { installPreviewHost } from '../../src/renderer/src/host/installPreviewHost'
import type { Message } from '../../src/shared/contracts'
import type { ModelCapability } from '../../src/renderer/src/features/models/useModelCapabilities'

const sampleModels: ModelCapability[] = [
  {
    provider_id: 'openai',
    provider_name: 'OpenAI',
    model_id: 'gpt-5.4',
    name: 'GPT-5.4',
    can_reason: true,
    reasoning_levels: ['off', 'low', 'medium', 'high', 'max'],
    default_reasoning_mode: 'medium'
  },
  {
    provider_id: 'anthropic',
    provider_name: 'Anthropic',
    model_id: 'claude-sonnet-4',
    name: 'Claude Sonnet 4',
    can_reason: true,
    reasoning_levels: ['off', 'low', 'medium', 'high'],
    default_reasoning_mode: 'medium'
  }
]

describe('orchestrator operation route & helpers', () => {
  it('builds POST config/orchestrator payload cleanly', () => {
    const [method, path, body] = operationRoute('orchestrator', {
      sessionId: 'sess-42',
      enabled: true,
      planner: { provider: 'openai', model: 'gpt-5.4', reasoning_mode: 'high' },
      executor: { provider: 'anthropic', model: 'claude-sonnet-4', reasoning_mode: 'medium' },
      finalReview: false
    })
    expect(method).toBe('POST')
    expect(path).toBe('config/orchestrator')
    expect(body).toEqual({
      scope: 0,
      session_id: 'sess-42',
      enabled: true,
      planner: { provider: 'openai', model: 'gpt-5.4', reasoning_mode: 'high' },
      executor: { provider: 'anthropic', model: 'claude-sonnet-4', reasoning_mode: 'medium' },
      final_review: false
    })
  })

  it('resolves planner and executor fallbacks and filters reasoning levels by capability', () => {
    const config = {
      models: {
        large: { provider: 'openai', model: 'gpt-5.4' },
        small: { provider: 'anthropic', model: 'claude-sonnet-4' }
      },
      session_models: {}
    }

    const planner = resolveRoleDisplay('planner', config, undefined, sampleModels)
    const executor = resolveRoleDisplay('executor', config, undefined, sampleModels)

    expect(planner.ref).toEqual({ provider: 'openai', model: 'gpt-5.4' })
    expect(planner.name).toBe('GPT-5.4')
    expect(planner.reasoningOptions).toEqual(['low', 'medium', 'high', 'max'])

    expect(executor.ref).toEqual({ provider: 'anthropic', model: 'claude-sonnet-4' })
    expect(executor.name).toBe('Claude Sonnet 4')
    expect(executor.reasoningOptions).toEqual(['off', 'low', 'medium', 'high'])
    expect(compactModelLabel(executor.name)).toBe('Sonnet 4')

    expect(resolveRoleConfig('planner', { ...config, session_models: { s1: { provider: 'anthropic', model: 'claude-sonnet-4' } } }, 's1')).toEqual({
      provider: 'anthropic',
      model: 'claude-sonnet-4'
    })
    expect(roleReasoningOptions('planner', sampleModels[1])).toEqual(['low', 'medium', 'high'])
    expect(resolveRoleReasoning('planner', undefined, sampleModels[0])).toBe('medium')
  })

  it('computes orchestrator active state and planner context window accurately', () => {
    expect(isOrchestratorActive(true, 'general', 'project')).toBe(true)
    expect(isOrchestratorActive(true, 'plan', 'project')).toBe(false)
    expect(isOrchestratorActive(true, 'general', 'designer')).toBe(false)
    expect(isOrchestratorActive(false, 'general', 'project')).toBe(false)

    expect(
      resolveContextWindow({
        orchestratorActive: true,
        plannerRef: { context_window: 1000000 },
        plannerCapability: { provider_id: 'openai', model_id: 'gpt-5.4', context_window: 400000 },
        selected: { context_window: 128000 },
        capability: { provider_id: 'anthropic', model_id: 'claude-sonnet-4', context_window: 200000 }
      })
    ).toBe(1000000)

    expect(
      resolveContextWindow({
        orchestratorActive: true,
        plannerCapability: { provider_id: 'openai', model_id: 'gpt-5.4', context_window: 400000 },
        selected: { context_window: 128000 }
      })
    ).toBe(400000)

    expect(
      resolveContextWindow({
        orchestratorActive: false,
        plannerRef: { context_window: 1000000 },
        selected: { context_window: 128000 }
      })
    ).toBe(128000)

    expect(
      hasExplicitRoleOverride(
        { orchestrator: { planner: { provider: 'openai', model: 'gpt-5.4' } } },
        'planner'
      )
    ).toBe(true)
    expect(
      hasExplicitRoleOverride(
        { orchestrator: { planner: { provider: '', model: '' } } },
        'planner'
      )
    ).toBe(false)
  })

  it('maps orchestrator phase labels in Russian', () => {
    const ru = (key: string) => translate('ru', key)
    expect(orchestratorPhaseLabel('plan', ru)).toBe('Планировщик')
    expect(orchestratorPhaseLabel('execute', ru)).toBe('Исполнитель')
    expect(orchestratorPhaseLabel('review', ru)).toBe('Проверка')
  })
})

describe('OrchestratorSettings UI', () => {
  it('renders switch, role cards, reasoning pills, and final review toggle in Russian', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <OrchestratorSettings
          enabled={true}
          finalReview={true}
          planner={{
            name: 'GPT-5.4',
            provider: 'OpenAI',
            reasoning: 'high',
            reasoningOptions: ['low', 'medium', 'high', 'max'],
            hasModel: true
          }}
          executor={{
            name: 'Claude Sonnet 4',
            provider: 'Anthropic',
            reasoning: 'medium',
            reasoningOptions: ['off', 'low', 'medium', 'high'],
            hasModel: true
          }}
          disabled={false}
          onToggleEnabled={vi.fn()}
          onToggleFinalReview={vi.fn()}
          onChangeReasoning={vi.fn()}
          onPickModel={vi.fn()}
        />
      </I18nProvider>
    )

    expect(html).toContain('Режим Оркестратора')
    expect(html).toContain('Планировщик проверяет, исполнитель пишет код')
    expect(html).toContain('Планировщик')
    expect(html).toContain('Исполнитель')
    expect(html).toContain('GPT-5.4')
    expect(html).toContain('Claude Sonnet 4')
    expect(html).toContain('orchestrator-role-trigger')
    expect(html).toContain('orchestrator-reasoning-segmented')
    expect(html).toContain('Финальная проверка')
    expect(html).toContain('перед ответом')
    expect(html).toContain('aria-checked="true"')
    expect(html).toContain('orchestrator-toggle-row is-active')
  })

  it('renders model picker row indicator, context pill, and reasoning badge cleanly', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ModelPickerRow
          model={{
            provider_id: 'openai',
            provider_name: 'OpenAI',
            model_id: 'gpt-5.4',
            name: 'GPT-5.4',
            context_window: 1049000,
            can_reason: true
          }}
          active={true}
          favorite={false}
          disabled={false}
          favoriteDisabled={false}
          showProvider={true}
          select={vi.fn()}
          toggleFavorite={vi.fn()}
        />
      </I18nProvider>
    )
    expect(html).toContain('model-picker-indicator is-active')
    expect(html).toContain('model-context-pill')
    expect(html).toContain('1049k')
    expect(html).toContain('model-reason-badge')
    expect(html).toContain('Поддерживает размышления')

    const enHtml = renderToStaticMarkup(
      <I18nProvider language="en">
        <ModelPickerRow
          model={{
            provider_id: 'anthropic',
            provider_name: 'Anthropic',
            model_id: 'claude-sonnet-4',
            name: 'Claude Sonnet 4',
            context_window: 256000,
            can_reason: true
          }}
          active={false}
          favorite={true}
          disabled={false}
          favoriteDisabled={false}
          showProvider={false}
          select={vi.fn()}
          toggleFavorite={vi.fn()}
        />
      </I18nProvider>
    )
    expect(enHtml).toContain('model-picker-indicator')
    expect(enHtml).not.toContain('model-picker-indicator is-active')
    expect(enHtml).toContain('256k')
    expect(enHtml).toContain('Supports reasoning')
  })

  it('renders auto badge on default roles and auto-reset option rows in both locales', () => {
    const settingsHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <OrchestratorSettings
          enabled={true}
          finalReview={true}
          planner={{
            name: 'GPT-5.4',
            provider: 'OpenAI',
            reasoning: 'high',
            reasoningOptions: ['low', 'medium', 'high', 'max'],
            hasModel: true,
            isDefault: true
          }}
          executor={{
            name: 'Claude Sonnet 4',
            provider: 'Anthropic',
            reasoning: 'medium',
            reasoningOptions: ['off', 'low', 'medium', 'high'],
            hasModel: true,
            isDefault: false
          }}
          disabled={false}
          onToggleEnabled={vi.fn()}
          onToggleFinalReview={vi.fn()}
          onChangeReasoning={vi.fn()}
          onPickModel={vi.fn()}
        />
      </I18nProvider>
    )
    expect(settingsHtml).toContain('orchestrator-role-badge')
    expect(settingsHtml).toContain('Авто')

    const ruAutoActiveHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <OrchestratorDefaultRow
          role="planner"
          active={true}
          disabled={false}
          fallbackName="GPT-5.4"
          onSelect={vi.fn()}
        />
      </I18nProvider>
    )
    expect(ruAutoActiveHtml).toContain('По умолчанию (авто)')
    expect(ruAutoActiveHtml).toContain('Использует модель чата / основную · GPT-5.4')
    expect(ruAutoActiveHtml).toContain('aria-selected="true"')
    expect(ruAutoActiveHtml).toContain('model-picker-indicator is-active')

    const enAutoInactiveHtml = renderToStaticMarkup(
      <I18nProvider language="en">
        <OrchestratorDefaultRow
          role="executor"
          active={false}
          disabled={false}
          fallbackName="Claude Sonnet 4"
          onSelect={vi.fn()}
        />
      </I18nProvider>
    )
    expect(enAutoInactiveHtml).toContain('Default (auto)')
    expect(enAutoInactiveHtml).toContain('Follows fast / primary model · Claude Sonnet 4')
    expect(enAutoInactiveHtml).toContain('aria-selected="false"')
  })
})

describe('ModelControl orchestrator visibility and context window', () => {
  const createWorkspaceMock = (overrides: {
    agentMode?: 'general' | 'plan'
    projectKind?: 'chat' | 'project' | 'designer'
  }) =>
    ({
      project: {
        id: 'ws-1',
        path: '/tmp/ws-1',
        clientId: 'c-1',
        connection: 'ready',
        kind: overrides.projectKind ?? 'project'
      },
      session: {
        id: 'sess-1',
        title: 'Test',
        prompt_tokens: 50000,
        completion_tokens: 0
      },
      preferences: {
        theme: 'dark',
        language: 'ru',
        recent: [],
        pinned: [],
        drafts: {},
        sidebarWidth: 256,
        inspectorWidth: 350,
        agentMode: overrides.agentMode ?? 'general'
      },
      reasoningMode: 'medium',
      orchestratorEnabled: true,
      config: {
        models: {
          large: { provider: 'anthropic', model: 'claude-sonnet-4', context_window: 200000 },
          small: { provider: 'anthropic', model: 'claude-haiku-4', context_window: 200000 }
        },
        orchestrator: {
          enabled: true,
          planner: { provider: 'openai', model: 'gpt-5.4', context_window: 1000000 },
          executor: { provider: 'anthropic', model: 'claude-sonnet-4', context_window: 200000 }
        }
      },
      agent: { model: { name: 'claude-sonnet-4' } },
      busy: false,
      call: vi.fn().mockResolvedValue([]),
      setReasoningMode: vi.fn(),
      savePreferences: vi.fn(),
      setOrchestrator: vi.fn(),
      errorHandler: vi.fn()
    }) as any

  it('shows orchestrator control and planner context window in general mode', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ModelControl workspace={createWorkspaceMock({ agentMode: 'general', projectKind: 'project' })} />
      </I18nProvider>
    )
    expect(html).toContain('model-control is-orchestrator')
    expect(html).toContain('Оркестратор')
    expect(html).toContain('5%')
    expect(html).toContain('1.0M')
  })

  it('suppresses orchestrator control in plan mode and designer workspaces', () => {
    const planHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ModelControl workspace={createWorkspaceMock({ agentMode: 'plan', projectKind: 'project' })} />
      </I18nProvider>
    )
    expect(planHtml).not.toContain('is-orchestrator')
    expect(planHtml).toContain('claude-sonnet-4')
    expect(planHtml).toContain('25%')
    expect(planHtml).toContain('200k')

    const designerHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ModelControl workspace={createWorkspaceMock({ agentMode: 'general', projectKind: 'designer' })} />
      </I18nProvider>
    )
    expect(designerHtml).not.toContain('is-orchestrator')
    expect(designerHtml).toContain('claude-sonnet-4')
    expect(designerHtml).toContain('25%')
  })
})

describe('MessageItem orchestrator phase indicator', () => {
  it('renders phase chip when assistant message starts or changes phase and suppresses duplicates', () => {
    const planMessage: Message = {
      id: 'm-plan',
      session_id: 's1',
      role: 'assistant',
      model: 'GPT-5.4',
      orchestrator_phase: 'plan',
      parts: [{ type: 'text', data: { text: 'Составляю план' } }]
    }
    const execMessage: Message = {
      id: 'm-exec',
      session_id: 's1',
      role: 'assistant',
      model: 'Claude Sonnet 4',
      orchestrator_phase: 'execute',
      parts: [{ type: 'text', data: { text: 'Пишу код' } }]
    }
    const reviewMessage: Message = {
      id: 'm-rev',
      session_id: 's1',
      role: 'assistant',
      model: 'GPT-5.4',
      orchestrator_phase: 'review',
      parts: [{ type: 'text', data: { text: 'Проверяю изменения' } }]
    }

    const planHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <MessageItem
          message={planMessage}
          continued={false}
          fresh={false}
          live={false}
          matched={false}
        />
      </I18nProvider>
    )
    expect(planHtml).toContain('orchestrator-phase-chip phase-plan')
    expect(planHtml).toContain('Планировщик · GPT-5.4')

    const execSamePhaseHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <MessageItem
          message={execMessage}
          continued={true}
          fresh={false}
          live={false}
          matched={false}
          previousOrchestratorPhase="execute"
        />
      </I18nProvider>
    )
    expect(execSamePhaseHtml).not.toContain('orchestrator-phase-chip')

    const execChangedPhaseHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <MessageItem
          message={execMessage}
          continued={true}
          fresh={false}
          live={false}
          matched={false}
          previousOrchestratorPhase="plan"
        />
      </I18nProvider>
    )
    expect(execChangedPhaseHtml).toContain('orchestrator-phase-chip phase-execute')
    expect(execChangedPhaseHtml).toContain('Исполнитель · Claude Sonnet 4')

    const reviewHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <MessageItem
          message={reviewMessage}
          continued={true}
          fresh={false}
          live={false}
          matched={false}
          previousOrchestratorPhase="execute"
        />
      </I18nProvider>
    )
    expect(reviewHtml).toContain('orchestrator-phase-chip phase-review')
    expect(reviewHtml).toContain('Проверка · GPT-5.4')
  })
})

describe('installPreviewHost orchestrator mock', () => {
  it('persists config.orchestrator and config.session_orchestrator in memory', async () => {
    const previousWindow = (globalThis as { window?: unknown }).window
    try {
      ;(globalThis as { window?: unknown }).window = { location: { search: '?auth=signed-in' } }
      installPreviewHost()
      const prime = (globalThis as { window: { prime: any } }).window.prime

      await prime.call('ws-prime', 'orchestrator', {
        sessionId: 'sess-1',
        enabled: true,
        planner: { provider: 'openai', model: 'gpt-5.4', reasoning_mode: 'xhigh' },
        executor: { provider: 'anthropic', model: 'claude-sonnet-4', reasoning_mode: 'low' },
        finalReview: false
      })

      const cfg = await prime.call('ws-prime', 'config')
      expect(cfg.session_orchestrator['sess-1']).toBe(true)
      expect(cfg.orchestrator.planner).toEqual({
        provider: 'openai',
        model: 'gpt-5.4',
        reasoning_mode: 'xhigh'
      })
      expect(cfg.orchestrator.executor).toEqual({
        provider: 'anthropic',
        model: 'claude-sonnet-4',
        reasoning_mode: 'low'
      })
      expect(cfg.orchestrator.final_review).toBe(false)

      await prime.call('ws-prime', 'orchestrator', {
        planner: { provider: '', model: '' },
        executor: { provider: '', model: '' }
      })
      const clearedCfg = await prime.call('ws-prime', 'config')
      expect(clearedCfg.orchestrator.planner).toBeUndefined()
      expect(clearedCfg.orchestrator.executor).toBeUndefined()
    } finally {
      if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window
      else (globalThis as { window?: unknown }).window = previousWindow
    }
  })
})

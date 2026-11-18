import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { operationRoute } from '../../src/shared/operations'
import { translate } from '../../src/shared/localization'
import {
  compactModelLabel,
  orchestratorPhaseLabel,
  resolveRoleConfig,
  resolveRoleDisplay,
  resolveRoleReasoning,
  roleReasoningOptions
} from '../../src/renderer/src/features/models/orchestrator'
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
    expect(html).toContain('Планировщик исследует и проверяет без правок, исполнитель пишет код')
    expect(html).toContain('Планировщик')
    expect(html).toContain('Исполнитель')
    expect(html).toContain('GPT-5.4')
    expect(html).toContain('Claude Sonnet 4')
    expect(html).toContain('Выбрать модель')
    expect(html).toContain('Финальная проверка плана')
    expect(html).toContain('aria-checked="true"')
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
    } finally {
      if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window
      else (globalThis as { window?: unknown }).window = previousWindow
    }
  })
})

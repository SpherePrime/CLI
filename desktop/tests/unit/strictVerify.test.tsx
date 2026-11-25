import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { operationRoute } from '../../src/shared/operations'
import { resolveStrictVerifyEnabled } from '../../src/renderer/src/features/chat/strictVerify'
import { StrictVerifyButton } from '../../src/renderer/src/features/chat/StrictVerifyButton'
import { StrictVerifyPanel } from '../../src/renderer/src/features/chat/StrictVerifyPanel'
import { ModelControl } from '../../src/renderer/src/features/models/ModelControl'
import { useQuickActions } from '../../src/renderer/src/app/useQuickActions'
import { installPreviewHost } from '../../src/renderer/src/host/installPreviewHost'
import type { Workspace } from '../../src/renderer/src/app/useWorkspace'
import type { WorkspaceConfig } from '../../src/shared/contracts'

function createWorkspaceMock(
  overrides: {
    strictVerifyEnabled?: boolean
    busy?: boolean
    hasProject?: boolean
    projectKind?: 'project' | 'designer'
    reasoningMode?: 'default' | 'off' | 'low' | 'medium' | 'high' | 'xhigh' | 'max'
  } = {}
): Workspace {
  const hasProject = overrides.hasProject ?? true
  return {
    project: hasProject
      ? {
          id: 'ws-1',
          path: '/repo',
          name: 'repo',
          kind: overrides.projectKind ?? 'project',
          connection: 'ready'
        }
      : null,
    session: { id: 'sess-1', title: 'Chat', prompt_tokens: 0, completion_tokens: 0 },
    busy: overrides.busy ?? false,
    strictVerifyEnabled: overrides.strictVerifyEnabled ?? false,
    setStrictVerify: vi.fn().mockResolvedValue(undefined),
    gameDevEnabled: false,
    setGameDev: vi.fn().mockResolvedValue(undefined),
    reasoningMode: overrides.reasoningMode ?? 'high',
    setReasoningMode: vi.fn().mockResolvedValue(undefined),
    orchestratorEnabled: false,
    setOrchestrator: vi.fn().mockResolvedValue(undefined),
    config: {
      models: { large: { provider: 'anthropic', model: 'claude-sonnet-4' } }
    },
    agent: { model: { name: 'Claude Sonnet 4' } },
    preferences: { theme: 'dark', agentMode: 'general' },
    savePreferences: vi.fn().mockResolvedValue(undefined),
    info: { runtime: 'ready' },
    messages: [],
    createSession: vi.fn(),
    openProject: vi.fn(),
    call: vi.fn().mockResolvedValue([]),
    errorHandler: vi.fn()
  } as unknown as Workspace
}

function QuickActionProbe({ workspace }: { workspace: Workspace }) {
  const actions = useQuickActions({
    workspace,
    openTools: vi.fn(),
    setModal: vi.fn(),
    openSearch: vi.fn(),
    focusMode: false,
    toggleFocus: vi.fn(),
    sidebarVisible: true,
    toggleSidebar: vi.fn(),
    inspectorVisible: true,
    toggleInspector: vi.fn()
  })
  const action = actions.find((item) => item.id === 'strictVerify')
  return (
    <div
      data-id={action?.id || ''}
      data-label={action?.label || ''}
      data-disabled={action?.disabled ? 'true' : 'false'}
    />
  )
}

describe('strictVerify operation routes', () => {
  it('routes strictVerify with sessionId and scope cleanly', () => {
    expect(operationRoute('strictVerify', { sessionId: 'sess-9', enabled: true })).toEqual([
      'POST',
      'config/strict-verify',
      { scope: 0, session_id: 'sess-9', enabled: true }
    ])
    expect(operationRoute('strictVerify', { scope: 1, enabled: false })).toEqual([
      'POST',
      'config/strict-verify',
      { scope: 1, enabled: false }
    ])
  })

  it('forwards strict_verify in send payload only when specified', () => {
    const [, , bodyOn] = operationRoute('send', {
      sessionId: 's1',
      prompt: 'test',
      strictVerify: true
    })
    expect(bodyOn).toMatchObject({ strict_verify: true })

    const [, , bodyDefault] = operationRoute('send', {
      sessionId: 's1',
      prompt: 'test'
    })
    expect(bodyDefault).not.toHaveProperty('strict_verify')
  })
})

describe('resolveStrictVerifyEnabled', () => {
  it('resolves global, session, and draft precedence', () => {
    expect(resolveStrictVerifyEnabled(undefined)).toBe(false)
    expect(resolveStrictVerifyEnabled({})).toBe(false)

    const globalOn: WorkspaceConfig = { options: { strict_verify: true } }
    expect(resolveStrictVerifyEnabled(globalOn)).toBe(true)
    expect(resolveStrictVerifyEnabled(globalOn, 'sess-1')).toBe(true)
    expect(resolveStrictVerifyEnabled(globalOn, undefined, false)).toBe(false)

    const sessionOverride: WorkspaceConfig = {
      options: { strict_verify: true },
      session_strict_verify: { 'sess-1': false, 'sess-2': true }
    }
    expect(resolveStrictVerifyEnabled(sessionOverride, 'sess-1')).toBe(false)
    expect(resolveStrictVerifyEnabled(sessionOverride, 'sess-2')).toBe(true)
    expect(resolveStrictVerifyEnabled(sessionOverride, 'sess-3')).toBe(true)
  })
})

describe('StrictVerifyButton, StrictVerifyPanel, and compact ModelControl UI', () => {
  it('renders compact icon-only StrictVerifyButton with active dot when enabled', () => {
    const offHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <StrictVerifyButton workspace={createWorkspaceMock({ strictVerifyEnabled: false })} />
      </I18nProvider>
    )
    expect(offHtml).toContain('class="strict-verify-chip"')
    expect(offHtml).toContain('aria-label="Строгая проверка: Выключена"')
    expect(offHtml).not.toContain('strict-verify-chip-dot')

    const onHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <StrictVerifyButton workspace={createWorkspaceMock({ strictVerifyEnabled: true })} />
      </I18nProvider>
    )
    expect(onHtml).toContain('strict-verify-chip is-active')
    expect(onHtml).toContain('aria-label="Строгая проверка: Включена"')
    expect(onHtml).toContain('strict-verify-chip-dot')
  })

  it('renders StrictVerifyPanel in Russian and English', () => {
    const ruHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <StrictVerifyPanel enabled={false} saving={false} onClose={vi.fn()} onToggle={vi.fn()} />
      </I18nProvider>
    )
    expect(ruHtml).toContain('Строгая проверка')
    expect(ruHtml).toContain('Независимый ревьюер')
    expect(ruHtml).toContain('Цикл до идеала')
    expect(ruHtml).not.toContain('Умный порог на мелочи')
    expect(ruHtml).toContain('Включить')

    const enHtml = renderToStaticMarkup(
      <I18nProvider language="en">
        <StrictVerifyPanel enabled={true} saving={false} onClose={vi.fn()} onToggle={vi.fn()} />
      </I18nProvider>
    )
    expect(enHtml).toContain('Strict Verify')
    expect(enHtml).toContain('Independent reviewer')
    expect(enHtml).toContain('Fix-and-reverify loop')
    expect(enHtml).toContain('Disable')
  })

  it('omits reasoning text on ModelControl chip while preserving reasoning in tooltip and aria-label', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ModelControl workspace={createWorkspaceMock({ reasoningMode: 'high' })} />
      </I18nProvider>
    )
    expect(html).toContain('class="model-control-name"')
    expect(html).not.toContain('class="model-control-level"')
    expect(html).toContain('title="claude-sonnet-4 · Высокий"')
    expect(html).toContain('aria-label="claude-sonnet-4 · Уровень размышлений: Высокий"')
  })
})

describe('useQuickActions strictVerify toggle', () => {
  it('exposes strictVerify quick action label for off and on states', () => {
    const htmlOff = renderToStaticMarkup(
      <I18nProvider language="ru">
        <QuickActionProbe workspace={createWorkspaceMock({ strictVerifyEnabled: false })} />
      </I18nProvider>
    )
    expect(htmlOff).toContain('data-id="strictVerify"')
    expect(htmlOff).toContain('data-label="Включить строгую проверку"')

    const htmlOn = renderToStaticMarkup(
      <I18nProvider language="ru">
        <QuickActionProbe workspace={createWorkspaceMock({ strictVerifyEnabled: true })} />
      </I18nProvider>
    )
    expect(htmlOn).toContain('data-label="Выключить строгую проверку"')
  })
})

describe('installPreviewHost strictVerify mock', () => {
  it('stores per-session and global strict_verify flags', async () => {
    const prevWindow = (globalThis as { window?: unknown }).window
    const mockWindow: Record<string, unknown> = {
      location: { search: '' },
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn()
    }
    ;(globalThis as { window?: unknown }).window = mockWindow
    try {
      installPreviewHost()
      const prime = mockWindow.prime as {
        call: (wsId: string, op: string, args?: Record<string, unknown>) => Promise<Record<string, unknown>>
      }
      const sessionRes = await prime.call('ws-prime', 'strictVerify', {
        sessionId: 'sess-a',
        enabled: true
      })
      expect(sessionRes.session_strict_verify).toMatchObject({ 'sess-a': true })

      const globalRes = await prime.call('ws-prime', 'strictVerify', { enabled: true })
      expect(globalRes.strict_verify).toBe(true)
    } finally {
      ;(globalThis as { window?: unknown }).window = prevWindow
    }
  })
})

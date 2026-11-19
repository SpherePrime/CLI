import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { operationRoute } from '../../src/shared/operations'
import { resolveGameDevEnabled } from '../../src/renderer/src/features/chat/gamedev'
import { GameDevButton } from '../../src/renderer/src/features/chat/GameDevButton'
import { GameDevPanel } from '../../src/renderer/src/features/chat/GameDevPanel'
import { useQuickActions } from '../../src/renderer/src/app/useQuickActions'
import { installPreviewHost } from '../../src/renderer/src/host/installPreviewHost'
import { createPreviewConfig } from '../../src/renderer/src/host/previewHostData'
import type { DesktopEvent, WorkspaceConfig } from '../../src/shared/contracts'

function createWorkspaceMock(overrides: {
  gameDevEnabled?: boolean
  busy?: boolean
  hasProject?: boolean
  projectKind?: 'project' | 'chat' | 'designer'
} = {}) {
  return {
    project:
      overrides.hasProject === false
        ? null
        : {
            id: 'ws-1',
            path: '/tmp/ws-1',
            clientId: 'c-1',
            connection: 'ready',
            kind: overrides.projectKind ?? ('project' as const)
          },
    session: {
      id: 'sess-1',
      title: 'Test'
    },
    messages: [],
    busy: overrides.busy ?? false,
    info: { runtime: 'ready' },
    gameDevEnabled: overrides.gameDevEnabled ?? false,
    setGameDev: vi.fn().mockResolvedValue(undefined),
    createSession: vi.fn(),
    openProject: vi.fn(),
    savePreferences: vi.fn(),
    errorHandler: vi.fn()
  } as any
}

function QuickActionsProbe({
  workspace,
  onTrigger
}: {
  workspace: any
  onTrigger?: (run: () => void) => void
}) {
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
  const action = actions.find((item) => item.id === 'gamedev')
  if (action && onTrigger) onTrigger(action.run)
  return (
    <div
      data-label={action?.label || ''}
      data-disabled={String(Boolean(action?.disabled))}
    />
  )
}

describe('gamedev operation routes', () => {
  it('routes gamedev with sessionId and scope cleanly', () => {
    expect(operationRoute('gamedev', { sessionId: 'sess-9', enabled: true })).toEqual([
      'POST',
      'config/gamedev',
      {
        scope: 0,
        session_id: 'sess-9',
        enabled: true
      }
    ])

    expect(operationRoute('gamedev', { scope: 1, enabled: false })).toEqual([
      'POST',
      'config/gamedev',
      {
        scope: 1,
        enabled: false
      }
    ])
  })

  it('maps optional gameDev flag on send operation', () => {
    const [, pathDefault, bodyDefault] = operationRoute('send', {
      sessionId: 'sess-1',
      prompt: 'Hello'
    })
    expect(pathDefault).toBe('agent')
    expect(bodyDefault).not.toHaveProperty('game_dev')

    const [, , bodyEnabled] = operationRoute('send', {
      sessionId: 'sess-1',
      prompt: 'Test level',
      gameDev: true
    })
    expect(bodyEnabled.game_dev).toBe(true)

    const [, , bodyDisabled] = operationRoute('send', {
      sessionId: 'sess-1',
      prompt: 'Test level',
      gameDev: false
    })
    expect(bodyDisabled.game_dev).toBe(false)
  })
})

describe('resolveGameDevEnabled', () => {
  it('defaults to false and respects global, session, and draft precedence', () => {
    expect(resolveGameDevEnabled(undefined)).toBe(false)
    expect(resolveGameDevEnabled({})).toBe(false)

    const globalEnabled: WorkspaceConfig = { options: { game_dev: true } }
    expect(resolveGameDevEnabled(globalEnabled)).toBe(true)
    expect(resolveGameDevEnabled(globalEnabled, 'sess-1')).toBe(true)
    expect(resolveGameDevEnabled(globalEnabled, undefined, false)).toBe(false)

    const sessionOverride: WorkspaceConfig = {
      options: { game_dev: true },
      session_game_dev: { 'sess-1': false, 'sess-2': true }
    }
    expect(resolveGameDevEnabled(sessionOverride, 'sess-1')).toBe(false)
    expect(resolveGameDevEnabled(sessionOverride, 'sess-2')).toBe(true)
    expect(resolveGameDevEnabled(sessionOverride, 'sess-3')).toBe(true)

    const globalDisabled: WorkspaceConfig = {
      options: { game_dev: false },
      session_game_dev: { 'sess-2': true }
    }
    expect(resolveGameDevEnabled(globalDisabled, 'sess-2')).toBe(true)
    expect(resolveGameDevEnabled(globalDisabled, 'sess-1')).toBe(false)
    expect(resolveGameDevEnabled(globalDisabled, undefined, true)).toBe(true)
  })
})

describe('GameDevButton and GameDevPanel UI', () => {
  it('renders dialog trigger chip in inactive and active states without auto-toggling', () => {
    const inactiveWs = createWorkspaceMock({ gameDevEnabled: false })
    const inactiveHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <GameDevButton workspace={inactiveWs} />
      </I18nProvider>
    )
    expect(inactiveHtml).toContain('class="gamedev-chip "')
    expect(inactiveHtml).toContain('aria-haspopup="dialog"')
    expect(inactiveHtml).toContain('aria-expanded="false"')
    expect(inactiveHtml).toContain('data-active="false"')
    expect(inactiveHtml).toContain('aria-label="Режим GameDev: Выключен"')
    expect(inactiveHtml).toContain('GameDev')
    expect(inactiveHtml).toContain('автотест-ботов')
    expect(inactiveHtml).not.toContain('disabled=""')
    expect(inactiveWs.setGameDev).not.toHaveBeenCalled()

    const activeWs = createWorkspaceMock({ gameDevEnabled: true })
    const activeHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <GameDevButton workspace={activeWs} />
      </I18nProvider>
    )
    expect(activeHtml).toContain('gamedev-chip is-active')
    expect(activeHtml).toContain('data-active="true"')
    expect(activeHtml).toContain('aria-label="Режим GameDev: Включён"')
    expect(activeWs.setGameDev).not.toHaveBeenCalled()
  })

  it('disables trigger when busy or without an open project and translates to English', () => {
    const busyHtml = renderToStaticMarkup(
      <I18nProvider language="en">
        <GameDevButton workspace={createWorkspaceMock({ busy: true })} />
      </I18nProvider>
    )
    expect(busyHtml).toContain('disabled=""')
    expect(busyHtml).toContain('aria-label="GameDev mode: Disabled"')
    expect(busyHtml).toContain('automated playthrough test bots')

    const noProjectHtml = renderToStaticMarkup(
      <I18nProvider language="en">
        <GameDevButton workspace={createWorkspaceMock({ hasProject: false, gameDevEnabled: true })} />
      </I18nProvider>
    )
    expect(noProjectHtml).toContain('disabled=""')
    expect(noProjectHtml).toContain('aria-label="GameDev mode: Enabled"')
  })

  it('renders GameDevPanel explanation, status pill, and footer buttons in Russian and English', () => {
    const offRuHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <GameDevPanel enabled={false} saving={false} onClose={vi.fn()} onToggle={vi.fn()} />
      </I18nProvider>
    )
    expect(offRuHtml).toContain('Режим GameDev')
    expect(offRuHtml).toContain('Выключен')
    expect(offRuHtml).toContain('Тестовые боты')
    expect(offRuHtml).toContain('Поиск багов')
    expect(offRuHtml).toContain('Проверка сложности')
    expect(offRuHtml).toContain('Когда режим выключен, инструкции для игр не добавляются в промпт.')
    expect(offRuHtml).toContain('class="quiet"')
    expect(offRuHtml).toContain('Закрыть')
    expect(offRuHtml).toContain('class="primary"')
    expect(offRuHtml).toContain('Включить')

    const onRuHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <GameDevPanel enabled={true} saving={false} onClose={vi.fn()} onToggle={vi.fn()} />
      </I18nProvider>
    )
    expect(onRuHtml).toContain('gamedev-status-pill is-active')
    expect(onRuHtml).toContain('Включён')
    expect(onRuHtml).toContain('class="quiet danger"')
    expect(onRuHtml).toContain('Выключить')

    const enHtml = renderToStaticMarkup(
      <I18nProvider language="en">
        <GameDevPanel enabled={false} saving={false} onClose={vi.fn()} onToggle={vi.fn()} />
      </I18nProvider>
    )
    expect(enHtml).toContain('GameDev mode')
    expect(enHtml).toContain('Disabled')
    expect(enHtml).toContain('Test bots')
    expect(enHtml).toContain('Bug detection')
    expect(enHtml).toContain('Difficulty check')
    expect(enHtml).toContain('Close')
    expect(enHtml).toContain('Enable')
  })
})

describe('useQuickActions gamedev toggle', () => {
  it('exposes enable/disable label, toggles state, and disables in designer or busy', () => {
    const wsOff = createWorkspaceMock({ gameDevEnabled: false })
    let runEnable: (() => void) | undefined
    const htmlOff = renderToStaticMarkup(
      <I18nProvider language="ru">
        <QuickActionsProbe workspace={wsOff} onTrigger={(run) => (runEnable = run)} />
      </I18nProvider>
    )
    expect(htmlOff).toContain('data-label="Включить режим GameDev"')
    expect(htmlOff).toContain('data-disabled="false"')
    runEnable?.()
    expect(wsOff.setGameDev).toHaveBeenCalledWith(true)

    const wsOn = createWorkspaceMock({ gameDevEnabled: true })
    let runDisable: (() => void) | undefined
    const htmlOn = renderToStaticMarkup(
      <I18nProvider language="ru">
        <QuickActionsProbe workspace={wsOn} onTrigger={(run) => (runDisable = run)} />
      </I18nProvider>
    )
    expect(htmlOn).toContain('data-label="Выключить режим GameDev"')
    runDisable?.()
    expect(wsOn.setGameDev).toHaveBeenCalledWith(false)

    const htmlDesigner = renderToStaticMarkup(
      <I18nProvider language="ru">
        <QuickActionsProbe workspace={createWorkspaceMock({ projectKind: 'designer' })} />
      </I18nProvider>
    )
    expect(htmlDesigner).toContain('data-disabled="true"')
  })
})

describe('installPreviewHost gamedev mock', () => {
  it('defaults game_dev to false and persists session/global toggles', async () => {
    const previewDefaults = createPreviewConfig()
    expect(previewDefaults.options.game_dev).toBe(false)
    expect(previewDefaults.session_game_dev).toEqual({})

    const previousWindow = (globalThis as { window?: unknown }).window
    try {
      ;(globalThis as { window?: unknown }).window = { location: { search: '?auth=signed-in' } }
      installPreviewHost()
      const prime = (globalThis as { window: { prime: any } }).window.prime
      const events: DesktopEvent[] = []
      const unsubscribe = prime.onEvent((event: DesktopEvent) => events.push(event))

      const initialCfg = await prime.call('ws-prime', 'config')
      expect(initialCfg.options.game_dev).toBe(false)

      const sessionRes = await prime.call('ws-prime', 'gamedev', {
        sessionId: 'sess-1',
        enabled: true
      })
      expect(sessionRes.session_game_dev['sess-1']).toBe(true)

      const globalRes = await prime.call('ws-prime', 'gamedev', {
        enabled: true
      })
      expect(globalRes.game_dev).toBe(true)

      const updatedCfg = await prime.call('ws-prime', 'config')
      expect(updatedCfg.session_game_dev['sess-1']).toBe(true)
      expect(updatedCfg.options.game_dev).toBe(true)
      expect(events.filter((e) => 'type' in e && e.type === 'config_changed')).toHaveLength(2)

      unsubscribe()
    } finally {
      ;(globalThis as { window?: unknown }).window = previousWindow
    }
  })
})

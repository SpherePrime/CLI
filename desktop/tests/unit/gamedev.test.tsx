import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { operationRoute } from '../../src/shared/operations'
import { resolveGameDevEnabled } from '../../src/renderer/src/features/chat/gamedev'
import { GameDevButton } from '../../src/renderer/src/features/chat/GameDevButton'
import { installPreviewHost } from '../../src/renderer/src/host/installPreviewHost'
import { createPreviewConfig } from '../../src/renderer/src/host/previewHostData'
import type { DesktopEvent, WorkspaceConfig } from '../../src/shared/contracts'

function createWorkspaceMock(overrides: {
  gameDevEnabled?: boolean
  busy?: boolean
  hasProject?: boolean
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
            kind: 'project' as const
          },
    session: {
      id: 'sess-1',
      title: 'Test'
    },
    busy: overrides.busy ?? false,
    gameDevEnabled: overrides.gameDevEnabled ?? false,
    setGameDev: vi.fn().mockResolvedValue(undefined),
    errorHandler: vi.fn()
  } as any
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

describe('GameDevButton UI', () => {
  it('renders inactive and active states with aria-pressed and Russian labels', () => {
    const inactiveHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <GameDevButton workspace={createWorkspaceMock({ gameDevEnabled: false })} />
      </I18nProvider>
    )
    expect(inactiveHtml).toContain('class="gamedev-chip "')
    expect(inactiveHtml).toContain('aria-pressed="false"')
    expect(inactiveHtml).toContain('aria-label="Включить режим GameDev"')
    expect(inactiveHtml).toContain('GameDev')
    expect(inactiveHtml).toContain('автотест-ботов')
    expect(inactiveHtml).not.toContain('disabled=""')

    const activeHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <GameDevButton workspace={createWorkspaceMock({ gameDevEnabled: true })} />
      </I18nProvider>
    )
    expect(activeHtml).toContain('gamedev-chip is-active')
    expect(activeHtml).toContain('aria-pressed="true"')
    expect(activeHtml).toContain('aria-label="Выключить режим GameDev"')
  })

  it('disables toggle when busy or without an open project and translates to English', () => {
    const busyHtml = renderToStaticMarkup(
      <I18nProvider language="en">
        <GameDevButton workspace={createWorkspaceMock({ busy: true })} />
      </I18nProvider>
    )
    expect(busyHtml).toContain('disabled=""')
    expect(busyHtml).toContain('aria-label="Enable GameDev mode"')
    expect(busyHtml).toContain('automated playthrough test bots')

    const noProjectHtml = renderToStaticMarkup(
      <I18nProvider language="en">
        <GameDevButton workspace={createWorkspaceMock({ hasProject: false, gameDevEnabled: true })} />
      </I18nProvider>
    )
    expect(noProjectHtml).toContain('disabled=""')
    expect(noProjectHtml).toContain('aria-label="Disable GameDev mode"')
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

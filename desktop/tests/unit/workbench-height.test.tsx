import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { WorkbenchDock } from '../../src/renderer/src/features/workbench/WorkbenchDock'
import {
  clampWorkbenchHeight,
  defaultWorkbenchHeight,
  workbenchHeightBounds
} from '../../src/renderer/src/features/workbench/workbenchHeight'
import type { Workspace } from '../../src/renderer/src/app/useWorkspace'

describe('workbench height persistence and bounds', () => {
  it('clamps persisted height to global bounds and viewport availability', () => {
    expect(clampWorkbenchHeight(80)).toBe(workbenchHeightBounds.min)
    expect(clampWorkbenchHeight(1600)).toBe(workbenchHeightBounds.max)
    expect(clampWorkbenchHeight(420.6)).toBe(421)
    expect(clampWorkbenchHeight(500, 320)).toBe(320)
    expect(clampWorkbenchHeight(Number.NaN)).toBe(workbenchHeightBounds.default)
    expect(defaultWorkbenchHeight(900)).toBe(340)
    expect(defaultWorkbenchHeight(500)).toBe(210)
  })

  it('restores saved workbenchHeight and exposes separator aria bounds', () => {
    const previousWindow = (globalThis as { window?: unknown }).window
    ;(globalThis as { window?: unknown }).window = {
      innerHeight: 900,
      prime: { onEvent: () => () => {} }
    }
    try {
      const workspace = {
        project: { id: 'ws-1', path: '/repo', clientId: 'c1', connection: 'ready' },
        session: null,
        preferences: {
          theme: 'dark',
          language: 'ru',
          recent: [],
          pinned: [],
          drafts: {},
          sidebarWidth: 256,
          inspectorWidth: 350,
          workbenchHeight: 295
        },
        savePreferences: vi.fn(),
        errorHandler: vi.fn()
      } as unknown as Workspace

      const markup = renderToStaticMarkup(
        <I18nProvider language="ru">
          <WorkbenchDock
            workspace={workspace}
            expanded={false}
            view="github"
            setView={() => {}}
            setExpanded={() => {}}
            close={() => {}}
          />
        </I18nProvider>
      )

      expect(markup).toContain('role="separator"')
      expect(markup).toContain('aria-orientation="horizontal"')
      expect(markup).toContain('aria-valuemin="140"')
      expect(markup).toContain('aria-valuemax="340"')
      expect(markup).toContain('aria-valuenow="295"')
      expect(markup).toContain('height:295px')
    } finally {
      if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window
      else (globalThis as { window?: unknown }).window = previousWindow
    }
  })
})

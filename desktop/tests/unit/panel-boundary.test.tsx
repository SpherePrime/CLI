import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { PanelBoundary } from '../../src/renderer/src/components/PanelBoundary'
import { Inspector } from '../../src/renderer/src/features/files/Inspector'
import type { Workspace } from '../../src/renderer/src/app/useWorkspace'

function renderBoundaryFallback(language: 'ru' | 'en', compact: boolean) {
  const boundary = new PanelBoundary({ compact, resetKey: 'a', children: null })
  boundary.state = {
    error: PanelBoundary.getDerivedStateFromError(new Error('Panel boom')).error,
    resetKey: 'a'
  }
  return renderToStaticMarkup(
    <I18nProvider language={language}>{boundary.render()}</I18nProvider>
  )
}

describe('panel boundary compact layout behavior', () => {
  it('captures errors and resets when resetKey changes', () => {
    const derived = PanelBoundary.getDerivedStateFromError(new Error('Panel boom'))
    expect(derived.error.message).toBe('Panel boom')

    expect(
      PanelBoundary.getDerivedStateFromProps(
        { resetKey: 'a', children: null },
        { error: derived.error, resetKey: 'a' }
      )
    ).toBeNull()

    expect(
      PanelBoundary.getDerivedStateFromProps(
        { resetKey: 'b', children: null },
        { error: derived.error, resetKey: 'a' }
      )
    ).toEqual({ error: null, resetKey: 'b' })
  })

  it('renders panel failure fallback cleanly for compact and desktop modes', () => {
    const compactMarkup = renderBoundaryFallback('ru', true)
    expect(compactMarkup).toContain('class="panel-failure"')
    expect(compactMarkup).toContain('class="empty-state compact"')
    expect(compactMarkup).toContain('Panel boom')

    const desktopMarkup = renderBoundaryFallback('en', false)
    expect(desktopMarkup).toContain('class="panel-failure"')
    expect(desktopMarkup).toContain('class="empty-state "')
    expect(desktopMarkup).toContain('This panel failed to open')
  })

  it('mounts inspector content container inside open compact inspector rail', () => {
    const workspace = {
      project: null,
      session: null,
      busy: false,
      config: {},
      preferences: {
        theme: 'dark',
        language: 'ru',
        recent: [],
        pinned: [],
        drafts: {},
        sidebarWidth: 256,
        inspectorWidth: 350
      },
      call: async () => [],
      send: async () => true,
      errorHandler: () => {}
    } as unknown as Workspace

    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <Inspector
          workspace={workspace}
          openDocument={async () => {}}
          compact
          compactOpen
          setExpanded={() => {}}
        />
      </I18nProvider>
    )

    expect(markup).toContain('inspector-rail panel-open')
    expect(markup).toContain('class="inspector-content')
  })
})

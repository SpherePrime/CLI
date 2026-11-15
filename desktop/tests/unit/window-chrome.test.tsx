import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { MouseEvent } from 'react'

const invokeMock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))

vi.mock('@tauri-apps/api/core', () => ({
  invoke: invokeMock
}))

import { I18nProvider } from '../../src/renderer/src/i18n'
import { dragNativeWindow, NativeWindowControls } from '../../src/renderer/src/host/windowChrome'

function createMouseEvent(matchingSelectors: string[], button = 0, detail = 1) {
  const preventDefault = vi.fn()
  const event = {
    button,
    detail,
    preventDefault,
    target: {
      closest: (selector: string) => {
        const selectors = selector.split(',').map((item) => item.trim())
        return selectors.some((item) => matchingSelectors.includes(item)) ? ({} as Element) : null
      }
    }
  } as unknown as MouseEvent<HTMLElement>
  return { event, preventDefault }
}

describe('NativeWindowControls and dragNativeWindow', () => {
  beforeEach(() => {
    invokeMock.mockClear()
  })

  it('renders minimize, maximize, and close controls with type="button"', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <NativeWindowControls />
      </I18nProvider>
    )
    const buttons = markup.match(/<button\b[^>]*>/g) ?? []
    expect(buttons).toHaveLength(3)
    for (const button of buttons) {
      expect(button).toContain('type="button"')
    }
    expect(markup).toContain('class="native-window-controls"')
    expect(markup).toContain('class="native-window-close"')
    expect(markup).toContain('aria-label="Свернуть"')
    expect(markup).toContain('aria-label="Развернуть"')
    expect(markup).toContain('aria-label="Закрыть"')
  })

  it('starts window drag on empty titlebar and maximizes on double click', () => {
    const single = createMouseEvent([], 0, 1)
    dragNativeWindow(single.event)
    expect(single.preventDefault).toHaveBeenCalledTimes(1)
    expect(invokeMock).toHaveBeenLastCalledWith('desktop_window', { action: 'drag' })

    const double = createMouseEvent([], 0, 2)
    dragNativeWindow(double.event)
    expect(double.preventDefault).toHaveBeenCalledTimes(1)
    expect(invokeMock).toHaveBeenLastCalledWith('desktop_window', { action: 'maximize' })
  })

  it('does not start window drag when clicking window controls or titlebar window space', () => {
    const onButton = createMouseEvent(['button', '.native-window-controls'], 0, 1)
    dragNativeWindow(onButton.event)
    expect(onButton.preventDefault).not.toHaveBeenCalled()

    const onSpace = createMouseEvent(['.titlebar-window-space'], 0, 1)
    dragNativeWindow(onSpace.event)
    expect(onSpace.preventDefault).not.toHaveBeenCalled()

    expect(invokeMock).not.toHaveBeenCalled()
  })
})

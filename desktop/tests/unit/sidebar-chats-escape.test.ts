import { describe, expect, it, vi } from 'vitest'
import { handleContextMenuKeyDown } from '../../src/renderer/src/features/projects/contextMenuEscape'

describe('sidebar chats context menu escape handling', () => {
  it('stops propagation and closes the context menu on Escape', () => {
    const close = vi.fn()
    const event = {
      key: 'Escape',
      preventDefault: vi.fn(),
      stopPropagation: vi.fn(),
      stopImmediatePropagation: vi.fn()
    } as unknown as KeyboardEvent

    expect(handleContextMenuKeyDown(event, close)).toBe(true)
    expect(event.preventDefault).toHaveBeenCalledTimes(1)
    expect(event.stopPropagation).toHaveBeenCalledTimes(1)
    expect(event.stopImmediatePropagation).toHaveBeenCalledTimes(1)
    expect(close).toHaveBeenCalledTimes(1)
  })

  it('ignores non-Escape keys', () => {
    const close = vi.fn()
    const event = {
      key: 'Enter',
      preventDefault: vi.fn(),
      stopPropagation: vi.fn(),
      stopImmediatePropagation: vi.fn()
    } as unknown as KeyboardEvent

    expect(handleContextMenuKeyDown(event, close)).toBe(false)
    expect(event.preventDefault).not.toHaveBeenCalled()
    expect(close).not.toHaveBeenCalled()
  })
})

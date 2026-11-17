import { afterEach, describe, expect, it, vi } from 'vitest'
import { applyFileSuggestion, nextSuggestionIndex } from '../../src/renderer/src/features/chat/suggestions'
import {
  createMentionController,
  extractMentionQuery,
  toFileSuggestions
} from '../../src/renderer/src/features/chat/useFileMentions'

describe('composer @ file suggestions', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('wraps keyboard navigation index across suggestions', () => {
    expect(nextSuggestionIndex(0, 4, 'ArrowDown')).toBe(1)
    expect(nextSuggestionIndex(3, 4, 'ArrowDown')).toBe(0)
    expect(nextSuggestionIndex(0, 4, 'ArrowUp')).toBe(3)
    expect(nextSuggestionIndex(2, 4, 'ArrowUp')).toBe(1)
    expect(nextSuggestionIndex(0, 0, 'ArrowDown')).toBe(0)
  })

  it('replaces the active @ query token with the chosen file path', () => {
    expect(applyFileSuggestion('Check @app', 'src/app/App.tsx')).toBe('Check @src/app/App.tsx ')
    expect(applyFileSuggestion('@', 'README.md')).toBe('@README.md ')
  })

  it('extracts active @ query tokens and formats file suggestions', () => {
    expect(extractMentionQuery('Check @App')).toBe('app')
    expect(extractMentionQuery('Check @')).toBe('')
    expect(extractMentionQuery('Check @app ')).toBeNull()
    expect(
      toFileSuggestions(
        [
          { name: 'App.tsx', path: 'src/app/App.tsx', directory: false },
          { name: 'src', path: 'src', directory: true }
        ],
        'app',
        { file: 'Файл', directory: 'Папка' }
      )
    ).toEqual([{ label: '@src/app/App.tsx', description: 'Файл', file: 'src/app/App.tsx' }])
  })

  it('debounces queries, caches per workspace, and discards stale async responses', async () => {
    vi.useFakeTimers()
    const deferred = {
      resolve: null as ((value: { name: string; path: string; directory: boolean }[]) => void) | null
    }
    const searchFiles = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise<{ name: string; path: string; directory: boolean }[]>((resolve) => {
            deferred.resolve = resolve
          })
      )
      .mockResolvedValueOnce([{ name: 'Composer.tsx', path: 'src/Composer.tsx', directory: false }])
    const onResult = vi.fn()
    const controller = createMentionController({
      debounceMs: 120,
      cacheTtlMs: 15000,
      searchFiles,
      listFiles: vi.fn().mockResolvedValue([]),
      labels: { file: 'Файл', directory: 'Папка' },
      onResult
    })

    controller.update('ws-1', '@a')
    controller.update('ws-1', '@ap')
    vi.advanceTimersByTime(119)
    expect(searchFiles).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(searchFiles).toHaveBeenCalledTimes(1)
    expect(searchFiles).toHaveBeenLastCalledWith('ws-1', 'ap', 50)

    controller.update('ws-1', '@comp')
    vi.advanceTimersByTime(120)
    await Promise.resolve()
    await Promise.resolve()
    expect(onResult).toHaveBeenLastCalledWith([
      { label: '@src/Composer.tsx', description: 'Файл', file: 'src/Composer.tsx' }
    ])

    deferred.resolve?.([{ name: 'App.tsx', path: 'src/App.tsx', directory: false }])
    await Promise.resolve()
    await Promise.resolve()
    expect(onResult).toHaveBeenLastCalledWith([
      { label: '@src/Composer.tsx', description: 'Файл', file: 'src/Composer.tsx' }
    ])

    onResult.mockClear()
    controller.update('ws-1', '@comp')
    expect(searchFiles).toHaveBeenCalledTimes(2)
    expect(onResult).toHaveBeenCalledWith([
      { label: '@src/Composer.tsx', description: 'Файл', file: 'src/Composer.tsx' }
    ])

    controller.update('ws-2', '@comp')
    vi.advanceTimersByTime(120)
    expect(searchFiles).toHaveBeenCalledTimes(3)
    expect(searchFiles).toHaveBeenLastCalledWith('ws-2', 'comp', 50)
  })

  it('falls back to files() when searchFiles fails', async () => {
    vi.useFakeTimers()
    const onResult = vi.fn()
    const controller = createMentionController({
      debounceMs: 120,
      searchFiles: vi.fn().mockRejectedValue(new Error('unsupported')),
      listFiles: vi
        .fn()
        .mockResolvedValue([{ name: 'README.md', path: 'README.md', directory: false }]),
      labels: { file: 'Файл', directory: 'Папка' },
      onResult
    })

    controller.update('ws-1', '@read')
    vi.advanceTimersByTime(120)
    await Promise.resolve()
    await Promise.resolve()
    await Promise.resolve()
    expect(onResult).toHaveBeenLastCalledWith([
      { label: '@README.md', description: 'Файл', file: 'README.md' }
    ])
  })
})

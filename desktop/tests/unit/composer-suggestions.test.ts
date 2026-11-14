import { describe, expect, it } from 'vitest'
import { applyFileSuggestion, nextSuggestionIndex } from '../../src/renderer/src/features/chat/suggestions'

describe('composer @ file suggestions', () => {
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
})

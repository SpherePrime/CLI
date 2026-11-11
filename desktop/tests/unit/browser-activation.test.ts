import { describe, expect, it } from 'vitest'
import {
  isInspectorOpen,
  shouldActivateBrowserTab
} from '../../src/renderer/src/features/browser/browserActivation'

describe('browserActivation', () => {
  describe('isInspectorOpen', () => {
    it('returns compactPanelOpen directly when layout is compact', () => {
      expect(isInspectorOpen(true, true, true, false)).toBe(true)
      expect(isInspectorOpen(true, true, false, true)).toBe(true)
      expect(isInspectorOpen(true, false, true, false)).toBe(false)
      expect(isInspectorOpen(true, false, true, true)).toBe(false)
    })

    it('returns true when desktop inspector is visible and not collapsed', () => {
      expect(isInspectorOpen(false, false, true, false)).toBe(true)
      expect(isInspectorOpen(false, true, true, false)).toBe(true)
    })

    it('returns false when desktop inspector is collapsed', () => {
      expect(isInspectorOpen(false, false, true, true)).toBe(false)
      expect(isInspectorOpen(false, true, true, true)).toBe(false)
    })

    it('returns false when focusMode is active or inspector is hidden', () => {
      expect(isInspectorOpen(false, false, false, false)).toBe(false)
      expect(isInspectorOpen(false, false, false, true)).toBe(false)
    })
  })

  describe('shouldActivateBrowserTab', () => {
    it('returns false when inspector is collapsed or closed', () => {
      expect(
        shouldActivateBrowserTab({ action: 'activate', sessionId: 'chat-1' }, 'chat-1', false)
      ).toBe(false)
    })

    it('returns false when event payload belongs to a different chat session', () => {
      expect(
        shouldActivateBrowserTab({ action: 'activate', sessionId: 'chat-2' }, 'chat-1', true)
      ).toBe(false)
    })

    it('returns true when inspector is open and session matches', () => {
      expect(
        shouldActivateBrowserTab({ action: 'activate', sessionId: 'chat-1' }, 'chat-1', true)
      ).toBe(true)
    })

    it('returns true when action is activate and event is chat-agnostic', () => {
      expect(shouldActivateBrowserTab({ action: 'activate' }, 'chat-1', true)).toBe(true)
      expect(shouldActivateBrowserTab({ action: 'activate', sessionId: null }, 'chat-1', true)).toBe(
        true
      )
    })

    it('returns false for non-activate actions or missing payload', () => {
      expect(shouldActivateBrowserTab(undefined, 'chat-1', true)).toBe(false)
      expect(shouldActivateBrowserTab({ action: 'state' }, 'chat-1', true)).toBe(false)
      expect(
        shouldActivateBrowserTab({ action: 'navigate', sessionId: 'chat-1' }, 'chat-1', true)
      ).toBe(false)
      expect(
        shouldActivateBrowserTab({ action: 'resize', sessionId: 'chat-1' }, 'chat-1', true)
      ).toBe(false)
    })
  })
})

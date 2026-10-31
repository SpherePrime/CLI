import { describe, expect, it } from 'vitest'
import type { PickedAppElement } from '../../src/shared/appTesting'
import { appElementPrompt, locatorText } from '../../src/renderer/src/features/appTesting/PickedElementCard'

function picked(overrides: Partial<PickedAppElement['element']> = {}): PickedAppElement {
  return {
    element: {
      reference: 'n3',
      role: 'button',
      name: 'Save',
      value: null,
      bounds: { x: 120, y: 300, width: 50, height: 50 },
      states: ['enabled'],
      actions: ['invoke'],
      automationId: 'save-primary',
      className: 'Button',
      path: 'window[Prime]/button[Save]',
      source: 'uia',
      confidence: 'high',
      ...overrides
    },
    reference: 'n3',
    confidence: 'high',
    candidates: 1,
    window: 'Prime app UI fixture',
    attachmentId: 'attachment-1'
  }
}

describe('application element prompt', () => {
  it('describes provider, role, locator candidates and confidence', () => {
    const prompt = appElementPrompt(picked(), 'поправь отступ')
    expect(prompt).toContain('поправь отступ')
    expect(prompt).toContain('role: button')
    expect(prompt).toContain('automationId: `save-primary`')
    expect(prompt).toContain('provider: uia, confidence: high')
    expect(prompt).toContain('A screenshot of the element is attached.')
  })

  it('falls back to role and name when there is no automation id', () => {
    const element = picked({ automationId: null })
    expect(locatorText(element)).toBe('role=button, name="Save"')
  })

  it('marks low confidence and omits the screenshot note without an attachment', () => {
    const element = picked({ automationId: null })
    element.confidence = 'low'
    element.attachmentId = undefined
    const prompt = appElementPrompt(element, 'что это')
    expect(prompt).toContain('confidence: low')
    expect(prompt).not.toContain('screenshot of the element')
  })
})

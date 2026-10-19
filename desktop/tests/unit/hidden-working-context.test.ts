import { expect, it } from 'vitest'
import { visibleMessageText } from '../../src/shared/messages'

it('hides generated working context while retaining normal user XML and assistant explanations', () => {
  const internal =
    'Fix login\n\n<working-context>\nTask goal:\nKeep users signed in\n</working-context>'
  expect(visibleMessageText(internal, 'user')).toBe('Fix login')
  expect(visibleMessageText(internal, 'assistant')).toBe(internal)
  const xml = 'Explain this\n\n<working-context>\nexample\n</working-context>'
  expect(visibleMessageText(xml, 'user')).toBe(xml)
})

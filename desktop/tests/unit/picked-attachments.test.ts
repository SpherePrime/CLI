import { expect, it } from 'vitest'
import { PickedAttachments } from '../../src/renderer/src/features/browser/pickedAttachments'

it('releases replaced, removed, closed and late picker screenshots', () => {
  const stored = new Set(['first', 'second', 'third', 'late'])
  const attachments = new PickedAttachments((ids) => ids.forEach((id) => stored.delete(id)))
  attachments.replace(['first', 'second'])
  attachments.replace(['second'])
  expect([...stored]).toEqual(['second', 'third', 'late'])
  attachments.replace(['third'])
  expect([...stored]).toEqual(['third', 'late'])
  attachments.dispose()
  expect([...stored]).toEqual(['late'])
  expect(attachments.replace(['late'])).toBe(false)
  expect(stored.size).toBe(0)
})

it('keeps screenshots alive when the picker closes during an accepted send', () => {
  const stored = new Set(['sending', 'next'])
  const attachments = new PickedAttachments((ids) => ids.forEach((id) => stored.delete(id)))
  attachments.replace(['sending'])
  attachments.beginSend(['sending'])
  attachments.replace(['next'])
  expect([...stored]).toEqual(['sending', 'next'])
  attachments.dispose()
  expect([...stored]).toEqual(['sending'])
  attachments.endSend(['sending'])
  expect(stored.size).toBe(0)
})

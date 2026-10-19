import { expect, it } from 'vitest'
import { nextOccurrence, validateInput } from '../../src/main/automations/schedule'

it('schedules daily local time strictly after now', () => {
  const now = new Date(2026, 9, 19, 9, 0).getTime()
  const next = nextOccurrence({ type: 'daily', time: '09:00' }, now)!
  expect(new Date(next).getDate()).toBe(20)
  expect(new Date(next).getHours()).toBe(9)
})

it('selects only requested weekly local days', () => {
  const now = new Date(2026, 9, 19, 10, 0).getTime()
  const next = nextOccurrence({ type: 'weekly', time: '08:30', days: [1, 5] }, now)!
  expect(new Date(next).getDay()).toBe(5)
  expect(new Date(next).getHours()).toBe(8)
  expect(new Date(next).getMinutes()).toBe(30)
})

it.each([
  { type: 'interval', minutes: 0 },
  { type: 'interval', minutes: Infinity },
  { type: 'daily', time: '25:00' },
  { type: 'weekly', time: '09:00', days: [] },
  { type: 'weekly', time: '09:00', days: [7] },
  { type: 'filesChanged', cooldownMinutes: 0 },
  { type: 'runFailed', cooldownMinutes: NaN }
])('rejects invalid trigger %j', (trigger) => {
  expect(() =>
    validateInput({ title: 'Review', prompt: 'Check', enabled: true, trigger } as Parameters<
      typeof validateInput
    >[0])
  ).toThrow('automation.invalid')
})

it('treats event triggers as unscheduled', () => {
  expect(nextOccurrence({ type: 'filesChanged', cooldownMinutes: 1 }, Date.now())).toBeUndefined()
})

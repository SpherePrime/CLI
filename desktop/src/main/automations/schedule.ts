import type { AutomationInput, AutomationTrigger } from '../../shared/automations'

export function validateInput(input: AutomationInput) {
  if (
    !input ||
    typeof input.title !== 'string' ||
    !input.title.trim() ||
    input.title.length > 160 ||
    typeof input.prompt !== 'string' ||
    !input.prompt.trim() ||
    input.prompt.length > 100000 ||
    typeof input.enabled !== 'boolean'
  )
    throw new Error('automation.invalid')
  const trigger = input.trigger
  if (
    !trigger ||
    !['interval', 'daily', 'weekly', 'filesChanged', 'runFailed'].includes(trigger.type)
  )
    throw new Error('automation.invalid')
  if (
    trigger.type === 'interval' &&
    (!Number.isFinite(trigger.minutes) || trigger.minutes < 1 || trigger.minutes > 525600)
  )
    throw new Error('automation.invalid')
  if (
    (trigger.type === 'daily' || trigger.type === 'weekly') &&
    !/^([01]\d|2[0-3]):[0-5]\d$/.test(trigger.time)
  )
    throw new Error('automation.invalid')
  if (
    trigger.type === 'weekly' &&
    (!Array.isArray(trigger.days) ||
      !trigger.days.length ||
      trigger.days.some((day) => !Number.isInteger(day) || day < 0 || day > 6))
  )
    throw new Error('automation.invalid')
  if (
    (trigger.type === 'filesChanged' || trigger.type === 'runFailed') &&
    (!Number.isFinite(trigger.cooldownMinutes) || trigger.cooldownMinutes < 1)
  )
    throw new Error('automation.invalid')
}

export function nextOccurrence(trigger: AutomationTrigger, now: number): number | undefined {
  if (trigger.type === 'interval') return now + trigger.minutes * 60000
  if (trigger.type !== 'daily' && trigger.type !== 'weekly') return undefined
  const [hours, minutes] = trigger.time.split(':').map(Number)
  for (let offset = 0; offset <= 7; offset++) {
    const candidate = new Date(now)
    candidate.setDate(candidate.getDate() + offset)
    candidate.setHours(hours, minutes, 0, 0)
    if (
      candidate.getTime() > now &&
      (trigger.type === 'daily' || trigger.days.includes(candidate.getDay()))
    )
      return candidate.getTime()
  }
  throw new Error('automation.invalid')
}

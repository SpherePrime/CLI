import type { ModelCapability } from './useModelCapabilities'

export const reasoningLevels = ['off', 'low', 'medium', 'high', 'xhigh', 'max'] as const

const aliases: Record<string, string> = {
  none: 'off',
  disabled: 'off',
  'x-high': 'xhigh',
  'extra-high': 'xhigh',
  maximum: 'max'
}

function normalize(value?: string) {
  if (!value) return ''
  const mode = aliases[value] || value
  return (reasoningLevels as readonly string[]).includes(mode) ? mode : ''
}

export function currentReasoningMode(selected: any, capability?: ModelCapability) {
  const explicit = normalize(selected?.reasoning_mode)
  if (explicit) return explicit
  if (selected?.think) return 'medium'
  const legacy = normalize(selected?.reasoning_effort)
  if (legacy) return legacy
  return normalize(capability?.default_reasoning_mode) || 'off'
}

export function reasoningLabel(mode: string, t: (key: string) => string) {
  const labels: Record<string, string> = {
    off: t('Выключены'),
    low: t('Лёгкий'),
    medium: t('Средний'),
    high: t('Высокий'),
    xhigh: t('Очень высокий'),
    max: t('Максимальный')
  }
  return labels[mode] || mode
}

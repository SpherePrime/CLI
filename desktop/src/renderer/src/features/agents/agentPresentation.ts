import { workerActive, type AgentWorker } from '../../../../shared/agents'

export type WorkerRoleKind = 'code' | 'explore' | 'plan' | 'review' | 'general'

export function workerRoleKind(agent: string): WorkerRoleKind {
  const normalized = agent.trim().toLowerCase()
  if (normalized === 'code') return 'code'
  if (normalized === 'explore') return 'explore'
  if (normalized === 'plan') return 'plan'
  if (normalized === 'review' || normalized === 'verifier') return 'review'
  return 'general'
}

export function activeWorkerCount(workers: AgentWorker[]) {
  return workers.filter(workerActive).length
}

export function formatWorkerDuration(
  startedAt: number,
  updatedAt: number,
  active: boolean,
  language: 'ru' | 'en',
  nowSeconds = Math.floor(Date.now() / 1000)
) {
  if (!startedAt || startedAt <= 0) return ''
  const endSeconds = active ? Math.max(startedAt, nowSeconds) : Math.max(startedAt, updatedAt || startedAt)
  const totalSeconds = Math.max(0, Math.round(endSeconds - startedAt))
  if (totalSeconds < 60) {
    return language === 'en' ? `${totalSeconds}s` : `${totalSeconds}с`
  }
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes < 60) {
    if (seconds === 0) return language === 'en' ? `${minutes}m` : `${minutes}м`
    return language === 'en' ? `${minutes}m ${seconds}s` : `${minutes}м ${seconds}с`
  }
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  if (remainingMinutes === 0) return language === 'en' ? `${hours}h` : `${hours}ч`
  return language === 'en' ? `${hours}h ${remainingMinutes}m` : `${hours}ч ${remainingMinutes}м`
}

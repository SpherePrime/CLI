import type { AutomationRule, AutomationRun } from '../../shared/automations'
import { validateInput } from './schedule'

export type AutomationState = {
  version: 1
  rules: AutomationRule[]
  runs: AutomationRun[]
  ownedRunIds: string[]
}
export const activeRun = (run: AutomationRun) =>
  run.status === 'claimed' || run.status === 'running'
export function parseState(value: unknown): AutomationState {
  const state = value as AutomationState
  if (
    !state ||
    state.version !== 1 ||
    !Array.isArray(state.rules) ||
    !Array.isArray(state.runs) ||
    !Array.isArray(state.ownedRunIds) ||
    state.ownedRunIds.some((id) => typeof id !== 'string')
  )
    throw new Error('automation.corrupt')
  const ruleIds = new Set<string>()
  for (const rule of state.rules) {
    validateInput(rule)
    if (
      typeof rule.id !== 'string' ||
      !rule.id ||
      ruleIds.has(rule.id) ||
      typeof rule.root !== 'string' ||
      !rule.root ||
      !Number.isFinite(rule.createdAt) ||
      !Number.isFinite(rule.updatedAt) ||
      (rule.nextAt !== undefined && !Number.isFinite(rule.nextAt)) ||
      (rule.lastTriggeredAt !== undefined && !Number.isFinite(rule.lastTriggeredAt)) ||
      (rule.lastFingerprint !== undefined && typeof rule.lastFingerprint !== 'string')
    )
      throw new Error('automation.corrupt')
    ruleIds.add(rule.id)
  }
  const runIds = new Set<string>()
  const activeRuleIds = new Set<string>()
  for (const run of state.runs) {
    if (
      !run ||
      typeof run.id !== 'string' ||
      !run.id ||
      runIds.has(run.id) ||
      typeof run.ruleId !== 'string' ||
      typeof run.root !== 'string' ||
      typeof run.title !== 'string' ||
      !['manual', 'interval', 'daily', 'weekly', 'filesChanged', 'runFailed'].includes(
        run.trigger
      ) ||
      !Number.isFinite(run.createdAt) ||
      (run.finishedAt !== undefined && !Number.isFinite(run.finishedAt)) ||
      (run.workspaceId !== undefined && typeof run.workspaceId !== 'string') ||
      (run.sessionId !== undefined && typeof run.sessionId !== 'string') ||
      (run.error !== undefined && typeof run.error !== 'string') ||
      !['claimed', 'running', 'completed', 'failed', 'cancelled', 'interrupted'].includes(
        run.status
      )
    )
      throw new Error('automation.corrupt')
    if (activeRun(run)) {
      if (
        activeRuleIds.has(run.ruleId) ||
        !state.rules.some((rule) => rule.id === run.ruleId && rule.root === run.root)
      )
        throw new Error('automation.corrupt')
      activeRuleIds.add(run.ruleId)
    }
    runIds.add(run.id)
  }
  return state
}

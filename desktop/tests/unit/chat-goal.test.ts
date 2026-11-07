import { describe, expect, it } from 'vitest'
import { goalPhase, goalProgress, normalizeGoal } from '../../src/shared/chatGoal'
import { operationRoute } from '../../src/shared/operations'

const goal = { session_id: 's', text: 'Ship the app', status: 'active' as const }

describe('chat goal', () => {
  it('counts stages and picks the stage in progress', () => {
    const progress = goalProgress([
      { content: 'Plan', status: 'completed' },
      { content: 'Build', status: 'pending' },
      { content: 'Test', status: 'in_progress', active_form: 'Testing' }
    ])
    expect(progress).toMatchObject({ total: 3, completed: 1, current: 'Testing' })
    expect(progress.ratio).toBeCloseTo(1 / 3)
    expect(goalProgress(undefined)).toMatchObject({ total: 0, completed: 0, ratio: 0 })
  })

  it('derives the phase from goal, stages and run state', () => {
    const open = goalProgress([{ content: 'Build', status: 'pending' }])
    const closed = goalProgress([{ content: 'Build', status: 'completed' }])
    expect(goalPhase(null, open, true)).toBe('none')
    expect(goalPhase(goal, goalProgress([]), true)).toBe('planning')
    expect(goalPhase(goal, goalProgress([]), false)).toBe('paused')
    expect(goalPhase(goal, open, true)).toBe('working')
    expect(goalPhase(goal, open, false)).toBe('paused')
    expect(goalPhase(goal, closed, true)).toBe('verifying')
    expect(goalPhase({ ...goal, status: 'achieved' }, closed, false)).toBe('achieved')
  })

  it('ignores empty engine answers', () => {
    expect(normalizeGoal(null)).toBeNull()
    expect(normalizeGoal({ text: '  ' })).toBeNull()
    expect(normalizeGoal(goal)).toEqual(goal)
  })

  it('routes goal operations to the session goal endpoint', () => {
    expect(operationRoute('goal', { sessionId: 'a b' })).toEqual(['GET', 'sessions/a%20b/goal'])
    expect(operationRoute('setGoal', { sessionId: 's', text: 'x' })).toEqual([
      'PUT',
      'sessions/s/goal',
      { text: 'x' }
    ])
    expect(operationRoute('clearGoal', { sessionId: 's' })).toEqual(['DELETE', 'sessions/s/goal'])
  })
})

import { describe, expect, it } from 'vitest'
import type { Session } from '../../src/shared/contracts'
import {
  isPendingSessionId,
  keepPendingSessions,
  newPendingSession,
  pendingSessionsOfProject,
  realSessionId
} from '../../src/shared/pendingSession'

const real = (id: string): Session => ({ id, title: 'Чат' })

describe('pending chat sessions', () => {
  it('marks a chat that the engine has not created yet', () => {
    const placeholder = newPendingSession('Новый чат')
    expect(isPendingSessionId(placeholder.id)).toBe(true)
    expect(placeholder.title).toBe('Новый чат')
  })

  it('does not treat engine session ids as pending', () => {
    expect(isPendingSessionId('f68e358a-9353-4aa4-8e22-8c756bb64708')).toBe(false)
    expect(isPendingSessionId('')).toBe(false)
    expect(isPendingSessionId(undefined)).toBe(false)
    expect(isPendingSessionId(null)).toBe(false)
  })

  it('keeps a pending row that a refresh dropped', () => {
    const placeholder = newPendingSession('Новый чат')
    const merged = keepPendingSessions([real('a')], [placeholder, real('a')])
    expect(merged.map((item) => item.id)).toEqual([placeholder.id, 'a'])
  })

  it('does not duplicate a pending row that is already listed', () => {
    const placeholder = newPendingSession('Новый чат')
    const merged = keepPendingSessions([real('a'), placeholder], [placeholder, real('a')])
    expect(merged.map((item) => item.id)).toEqual(['a', placeholder.id])
  })

  it('returns the snapshot untouched when nothing is pending', () => {
    const snapshot = [real('a'), real('b')]
    expect(keepPendingSessions(snapshot, [real('a')])).toBe(snapshot)
  })

  it('never hands a pending id to the engine', () => {
    const placeholder = newPendingSession('Новый чат')
    expect(realSessionId(placeholder)).toBeUndefined()
    expect(realSessionId(real('f68e358a-9353-4aa4-8e22-8c756bb64708'))).toBe(
      'f68e358a-9353-4aa4-8e22-8c756bb64708'
    )
    expect(realSessionId(null)).toBeUndefined()
    expect(realSessionId(undefined)).toBeUndefined()
  })

  it('keeps a pending row only for the project that asked for it', () => {
    const placeholder = newPendingSession('Новый чат')
    const previous = [placeholder, real('a')]
    const projectOf = new Map([[placeholder.id, 'project-a']])
    expect(pendingSessionsOfProject(previous, projectOf, 'project-a')).toEqual([placeholder])
    expect(pendingSessionsOfProject(previous, projectOf, 'project-b')).toEqual([])
  })

  it('does not carry a pending row into another project list', () => {
    const placeholder = newPendingSession('Новый чат')
    const projectOf = new Map([[placeholder.id, 'project-a']])
    const merged = keepPendingSessions(
      [real('b')],
      pendingSessionsOfProject([placeholder, real('a')], projectOf, 'project-b')
    )
    expect(merged.map((item) => item.id)).toEqual(['b'])
  })
})

import { expect, it } from 'vitest'
import {
  orderedProjectPaths,
  orderedSessionIds,
  orderedSessions,
  reorderProjectPaths
} from '../../src/renderer/src/features/projects/projectOrder'

it('retains manual order when opening a project changes the recent list', () => {
  const order = reorderProjectPaths(['/a', '/b', '/c'], '/c', '/a', 'before')
  expect(orderedProjectPaths(['/a', '/c', '/b'], [], order)).toEqual(['/c', '/a', '/b'])
})

it('drops removed paths and adds new and open projects without duplicating paths', () => {
  expect(
    orderedProjectPaths(['/new', '/b', '/a'], ['/open', '/a'], ['/b', '/gone', '/a', '/b'])
  ).toEqual(['/b', '/a', '/new', '/open'])
})

it('moves in both directions without changing the input or losing hidden projects', () => {
  const paths = ['/a', '/hidden', '/b', '/c']
  expect(reorderProjectPaths(paths, '/a', '/b', 'after')).toEqual(['/hidden', '/b', '/a', '/c'])
  expect(reorderProjectPaths(paths, '/c', '/b', 'before')).toEqual(['/a', '/hidden', '/c', '/b'])
  expect(paths).toEqual(['/a', '/hidden', '/b', '/c'])
})

it('keeps chat positions stable when activity updates a chat', () => {
  const sessions = [
    { id: 'old', created_at: 100, updated_at: 999 },
    { id: 'new', created_at: 200, updated_at: 1 }
  ]
  expect(orderedSessions(sessions, [])).toEqual([
    { id: 'new', created_at: 200, updated_at: 1 },
    { id: 'old', created_at: 100, updated_at: 999 }
  ])
})

it('pins first, then follows the saved manual order, then creation time', () => {
  const sessions = [
    { id: 'a', created_at: 300 },
    { id: 'b', created_at: 200 },
    { id: 'c', created_at: 100 }
  ]
  expect(orderedSessions(sessions, [], ['c', 'a'])).toEqual([
    { id: 'c', created_at: 100 },
    { id: 'a', created_at: 300 },
    { id: 'b', created_at: 200 }
  ])
  expect(orderedSessions(sessions, ['b'], ['c'])).toEqual([
    { id: 'b', created_at: 200 },
    { id: 'c', created_at: 100 },
    { id: 'a', created_at: 300 }
  ])
})

it('merges a saved chat order with chats created after it was saved', () => {
  expect(orderedSessionIds(['s3', 's1', 's2'], ['s2'])).toEqual(['s2', 's3', 's1'])
})

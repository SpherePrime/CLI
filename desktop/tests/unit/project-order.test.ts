import { expect, it } from 'vitest'
import {
  orderedProjectPaths,
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

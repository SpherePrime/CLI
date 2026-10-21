import { expect, it } from 'vitest'
import { operationRoute } from '../../src/main/prime/operations'

it('scopes access mode updates and reads to the requested session', () => {
  expect(operationRoute('skip', { skip: false, sessionId: 'chat/one' })).toEqual([
    'POST',
    'permissions/skip',
    { skip: false, session_id: 'chat/one' }
  ])
  expect(operationRoute('getSkip', { sessionId: 'chat/one?two' })).toEqual([
    'GET',
    'permissions/skip?session_id=chat%2Fone%3Ftwo'
  ])
})

it('preserves workspace access mode routes when no session is supplied', () => {
  expect(operationRoute('getSkip')).toEqual(['GET', 'permissions/skip'])
  expect(operationRoute('skip', { skip: true })).toEqual([
    'POST',
    'permissions/skip',
    { skip: true, session_id: undefined }
  ])
})

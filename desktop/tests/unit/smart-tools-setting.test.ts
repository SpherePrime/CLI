import { describe, expect, it } from 'vitest'
import { operationRoute } from '../../src/shared/operations'

describe('Smart Tools saved setting', () => {
  for (const scope of [0, 1]) {
    for (const value of [true, false]) {
      it(`sends ${value} to config/set in scope ${scope}`, () => {
        expect(operationRoute('configSet', { scope, key: 'options.smart_tools', value })).toEqual([
          'POST',
          'config/set',
          { scope, key: 'options.smart_tools', value }
        ])
      })
    }
  }
})

import { expect, it, vi } from 'vitest'
import type { AutomationManager } from '../../src/main/automations/manager'
import { automationOperation } from '../../src/main/workbench/automationIpc'

it('binds requests to the selected project and rejects unknown actions', async () => {
  const handle = vi.fn(async (request) => request)
  const manager = { handle } as unknown as AutomationManager
  expect(
    await automationOperation(manager, 'C:/selected', { action: 'list', root: 'C:/foreign' })
  ).toEqual({ action: 'list', root: 'C:/selected' })
  expect(() =>
    automationOperation(manager, 'C:/selected', { action: 'execute', root: 'C:/foreign' })
  ).toThrow()
  expect(() =>
    automationOperation(manager, 'C:/selected', { action: 'run', id: '../../foreign' })
  ).toThrow()
  expect(handle).toHaveBeenCalledTimes(1)
})

it('refuses excessive prompts at the IPC boundary before scheduling work', () => {
  const handle = vi.fn()
  expect(() =>
    automationOperation({ handle } as unknown as AutomationManager, 'C:/selected', {
      action: 'save',
      input: { prompt: 'x'.repeat(200001) }
    })
  ).toThrow('слишком большой')
  expect(handle).not.toHaveBeenCalled()
})

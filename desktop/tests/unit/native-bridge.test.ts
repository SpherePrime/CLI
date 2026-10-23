import { describe, expect, it, vi } from 'vitest'
import {
  createNativeBridge,
  type NativeInvoke,
  type NativeListen
} from '../../src/renderer/src/host/nativeBridge'

describe('native host bridge', () => {
  it('preserves arguments and routes existing engine operations', async () => {
    const invoke = vi.fn().mockResolvedValue([])
    const bridge = await createNativeBridge(
      invoke as NativeInvoke,
      vi.fn().mockResolvedValue(vi.fn())
    )
    await bridge.api.files('workspace')
    expect(invoke).toHaveBeenLastCalledWith('desktop_call', {
      method: 'files',
      args: ['workspace']
    })
    await bridge.api.call('workspace', 'config')
    expect(invoke).toHaveBeenLastCalledWith('desktop_call', {
      method: 'call',
      args: ['workspace', 'config', null, ['GET', 'config']]
    })
    await bridge.api.importImages([{ name: 'image.png', data: new Uint8Array([1, 2, 255]) }])
    expect(invoke).toHaveBeenLastCalledWith('desktop_call', {
      method: 'importImages',
      args: [[{ name: 'image.png', data: 'AQL/' }]]
    })
    await bridge.api.call('workspace', 'messagePage', { sessionId: 'chat', limit: 60 })
    expect(invoke).toHaveBeenLastCalledWith('desktop_call', {
      method: 'call',
      args: [
        'workspace',
        'messagePage',
        { sessionId: 'chat', limit: 60 },
        ['GET', 'sessions/chat/messages?limit=60']
      ]
    })
    invoke.mockRejectedValueOnce({ code: 'not_available', message: 'Недоступно' })
    await expect(bridge.api.browserState()).rejects.toThrow('Недоступно')
    bridge.dispose()
  })

  it('unsubscribes individual listeners and releases native event registration', async () => {
    let receive: Parameters<NativeListen>[1]
    const release = vi.fn()
    const bridge = await createNativeBridge(vi.fn() as NativeInvoke, async (_name, listener) => {
      receive = listener
      return release
    })
    const first = vi.fn(),
      second = vi.fn()
    const unsubscribe = bridge.api.onEvent(first)
    bridge.api.onEvent(second)
    const payload = { workspaceId: 'workspace', type: 'message', payload: {} }
    receive!({ payload })
    unsubscribe()
    receive!({ payload })
    expect(first).toHaveBeenCalledTimes(1)
    expect(second).toHaveBeenCalledTimes(2)
    bridge.dispose()
    expect(release).toHaveBeenCalledTimes(1)
  })
})

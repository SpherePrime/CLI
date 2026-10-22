import { describe, expect, it, vi } from 'vitest'
import {
  createNativeBridge,
  type NativeInvoke,
  type NativeListen
} from '../../src/renderer/src/host/nativeBridge'
import { hostMethods } from '../../src/shared/hostMethods'
import { readFileSync } from 'node:fs'
import ts from 'typescript'

describe('native host bridge', () => {
  it('covers every current preload method', () => {
    const source = readFileSync(new URL('../../src/preload/index.ts', import.meta.url), 'utf8')
    const file = ts.createSourceFile('preload.ts', source, ts.ScriptTarget.Latest, true)
    const methods: string[] = []
    function visit(node: ts.Node) {
      if (
        ts.isVariableDeclaration(node) &&
        node.name.getText(file) === 'api' &&
        node.initializer &&
        ts.isObjectLiteralExpression(node.initializer)
      )
        methods.push(...node.initializer.properties.map((property) => property.name!.getText(file)))
      ts.forEachChild(node, visit)
    }
    visit(file)
    expect([...Object.keys(hostMethods), 'onEvent'].sort()).toEqual(methods.sort())
  })

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
    let receive: Parameters<Parameters<NativeListen>[1]>[0] extends never
      ? never
      : Parameters<NativeListen>[1]
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

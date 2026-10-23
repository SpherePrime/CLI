import type { DesktopApi, DesktopEvent, Operation } from '../../../shared/contracts'
import { hostMethods } from '../../../shared/hostMethods'
import { operationRoute } from '../../../shared/operations'

export type NativeInvoke = <T>(command: string, args: Record<string, unknown>) => Promise<T>
export type NativeListen = (
  event: string,
  listener: (event: { payload: DesktopEvent }) => void
) => Promise<() => void>

export async function createNativeBridge(invoke: NativeInvoke, listen: NativeListen) {
  const listeners = new Set<(event: DesktopEvent) => void>()
  const unlisten = await listen('prime:event', (event) => {
    for (const listener of [...listeners]) listener(event.payload)
  })
  const methods = Object.fromEntries(
    Object.keys(hostMethods).map((method) => [
      method,
      (...args: unknown[]) => {
        const normalized = args.map((value) => (value === undefined ? null : value))
        if (method === 'importImages' && Array.isArray(args[0])) {
          normalized[0] = args[0].map((image: { name: string; data: Uint8Array }) => {
            let binary = ''
            for (let offset = 0; offset < image.data.length; offset += 32768)
              binary += String.fromCharCode(...image.data.subarray(offset, offset + 32768))
            return { name: image.name, data: btoa(binary) }
          })
        }
        if (method === 'call') {
          while (normalized.length < 3) normalized.push(null)
          normalized.push(operationRoute(args[1] as Operation, args[2] ?? {}))
        }
        return invoke('desktop_call', { method, args: normalized }).catch((error: unknown) => {
          if (error instanceof Error) throw error
          const message =
            typeof error === 'string' ? error : (error as { message?: string })?.message
          throw new Error(message || 'Ошибка нативного приложения')
        })
      }
    ])
  )
  const api = {
    ...methods,
    onEvent(listener: (event: DesktopEvent) => void) {
      listeners.add(listener)
      return () => {
        listeners.delete(listener)
      }
    }
  } as DesktopApi
  return {
    api,
    dispose: () => {
      listeners.clear()
      unlisten()
    }
  }
}

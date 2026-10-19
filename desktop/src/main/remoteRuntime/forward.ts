import { createConnection, type Socket } from 'node:net'
import type { Client, ClientChannel, TcpConnectionDetails } from 'ssh2'

interface Binding {
  localPort: number
  connections: Set<() => void>
}

interface Registry {
  bindings: Map<number, Binding>
  pending: Set<(error: Error) => void>
  closing: Set<() => void>
  lost: boolean
  detach(): void
}

const registries = new WeakMap<Client, Registry>()

function registryFor(client: Client): Registry {
  const existing = registries.get(client)
  if (existing) return existing
  const registry: Registry = {
    bindings: new Map(),
    pending: new Set(),
    closing: new Set(),
    lost: false,
    detach: () => {
      if (registry.bindings.size || registry.pending.size || registry.closing.size) return
      client.removeListener('tcp connection', incoming)
      client.removeListener('close', lost)
      client.removeListener('end', lost)
      client.removeListener('error', lost)
      if (registries.get(client) === registry) registries.delete(client)
    }
  }
  const incoming = (
    details: TcpConnectionDetails,
    accept: () => ClientChannel,
    reject: () => void
  ) => {
    const binding = registry.bindings.get(details.destPort)
    if (registry.lost || !binding || details.destIP !== '127.0.0.1') {
      reject()
      return
    }
    let channel: ClientChannel
    let socket: Socket
    try {
      channel = accept()
      socket = createConnection({ host: '127.0.0.1', port: binding.localPort })
    } catch {
      return
    }
    const destroy = () => {
      binding.connections.delete(destroy)
      socket.destroy()
      channel.destroy()
    }
    binding.connections.add(destroy)
    socket.on('error', destroy).once('close', destroy)
    channel.on('error', destroy).once('close', destroy)
    socket.pipe(channel).pipe(socket)
  }
  const lost = () => {
    registry.lost = true
    for (const binding of registry.bindings.values())
      for (const destroy of binding.connections) destroy()
    registry.bindings.clear()
    for (const reject of registry.pending)
      reject(new Error('SSH connection closed during forwarding'))
    registry.pending.clear()
    for (const finish of registry.closing) finish()
    registry.closing.clear()
    registry.detach()
  }
  client.on('tcp connection', incoming).once('close', lost).once('end', lost).on('error', lost)
  registries.set(client, registry)
  return registry
}

export async function createRemoteForward(
  client: Client,
  localPort: number
): Promise<{ port: number; close(): Promise<void> }> {
  if (!Number.isInteger(localPort) || localPort < 1 || localPort > 65535)
    throw new Error('Invalid local forwarding port')
  const registry = registryFor(client)
  return new Promise((resolve, reject) => {
    const fail = (error: Error) => {
      registry.pending.delete(fail)
      registry.detach()
      reject(error)
    }
    registry.pending.add(fail)
    try {
      client.forwardIn('127.0.0.1', 0, (error, port) => {
        if (error) {
          fail(error)
          return
        }
        if (registry.lost) {
          fail(new Error('SSH connection closed during forwarding'))
          return
        }
        if (!Number.isInteger(port) || port < 1 || port > 65535 || registry.bindings.has(port)) {
          fail(new Error('Invalid remote forwarding port'))
          return
        }
        const binding: Binding = { localPort, connections: new Set() }
        registry.bindings.set(port, binding)
        registry.pending.delete(fail)
        let closing: Promise<void> | undefined
        resolve({
          port,
          close: () => {
            if (closing) return closing
            if (registry.lost || registry.bindings.get(port) !== binding) return Promise.resolve()
            registry.bindings.delete(port)
            for (const destroy of binding.connections) destroy()
            closing = new Promise<void>((done, failed) => {
              const finish = (error?: Error | null) => {
                registry.closing.delete(disconnected)
                registry.detach()
                if (error && !registry.lost) failed(error)
                else done()
              }
              const disconnected = () => finish()
              registry.closing.add(disconnected)
              try {
                client.unforwardIn('127.0.0.1', port, finish)
              } catch (error) {
                finish(error as Error)
              }
            })
            return closing
          }
        })
      })
    } catch (error) {
      fail(error as Error)
    }
  })
}

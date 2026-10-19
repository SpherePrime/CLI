import { createServer, type Server, type Socket } from 'node:net'
import type { Client } from 'ssh2'

export type LocalForward = { port: number; close(): Promise<void> }

export async function createLocalForward(client: Client, remotePort: number): Promise<LocalForward> {
  const sockets = new Set<Socket>()
  const server: Server = createServer((socket) => {
    sockets.add(socket)
    socket.once('close', () => sockets.delete(socket))
    client.forwardOut('127.0.0.1', 0, '127.0.0.1', remotePort, (error, channel) => {
      if (error) {
        socket.destroy()
        return
      }
      socket.pipe(channel).pipe(socket)
      channel.once('close', () => socket.destroy())
      socket.once('close', () => channel.close())
    })
  })
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => resolve())
  })
  const lost = () => void close()
  client.once('close', lost)
  let closing: Promise<void> | undefined
  const close = () => {
    closing ??= new Promise<void>((resolve) => {
      client.removeListener('close', lost)
      for (const socket of sockets) socket.destroy()
      server.close(() => resolve())
    })
    return closing
  }
  return { port: (server.address() as { port: number }).port, close }
}

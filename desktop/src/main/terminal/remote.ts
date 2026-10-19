import type { ClientChannel } from 'ssh2'
import type { RemoteConnection } from '../remote/connection'
import { shellQuote } from '../remote/security'
import type { TerminalExit, TerminalHandle } from './handle'

export async function remoteTerminalHandle(
  connection: RemoteConnection,
  cols: number,
  rows: number
): Promise<TerminalHandle> {
  const command = `cd ${shellQuote(connection.root)} && exec "\${SHELL:-/bin/sh}" -l`
  const channel = await new Promise<ClientChannel>((resolve, reject) =>
    connection.client.exec(
      command,
      { pty: { cols, rows, term: 'xterm-256color' }, env: { COLORTERM: 'truecolor' } },
      (error, stream) => (error ? reject(error) : resolve(stream))
    )
  )
  const decoder = new TextDecoder('utf-8')
  let exit: TerminalExit = { exitCode: -1 }
  let closed = false
  channel.on('exit', (code: number | null, signal?: string) => {
    exit = { exitCode: code ?? -1, ...(signal ? { signal: 1 } : {}) }
  })
  channel.on('close', () => {
    closed = true
  })
  return {
    pid: 0,
    write: (data) => channel.write(data),
    resize: (nextCols, nextRows) => channel.setWindow(nextRows, nextCols, 0, 0),
    onData: (listener) => {
      const handler = (chunk: Buffer) => listener(decoder.decode(chunk, { stream: true }))
      channel.on('data', handler)
      channel.stderr.on('data', handler)
      return {
        dispose: () => {
          channel.removeListener('data', handler)
          channel.stderr.removeListener('data', handler)
        }
      }
    },
    onExit: (listener) => {
      const handler = () => listener(exit)
      channel.once('close', handler)
      return { dispose: () => channel.removeListener('close', handler) }
    },
    stop: async (wasRunning) => {
      if (closed || !wasRunning) return
      const done = new Promise<void>((resolve) => channel.once('close', () => resolve()))
      channel.signal('HUP')
      channel.end()
      await Promise.race([done, new Promise((resolve) => setTimeout(resolve, 3000))])
      if (!closed) channel.close()
    }
  }
}
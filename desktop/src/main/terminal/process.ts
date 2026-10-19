import type { IPty } from 'node-pty'
import type { Worker } from 'node:worker_threads'

type WindowsPtyResources = {
  _agent?: {
    _conoutSocketWorker?: {
      dispose(): void
      connectSocket(socket: unknown): void
      _worker?: Worker
    }
    inSocket?: { destroy(): void }
    outSocket?: { destroy(): void }
  }
}

export async function releaseTerminalResources(processHandle: IPty) {
  if (process.platform !== 'win32') return
  const agent = (processHandle as IPty & WindowsPtyResources)._agent
  const connection = agent?._conoutSocketWorker
  if (connection) {
    connection.connectSocket = () => {}
    connection.dispose()
  }
  agent?.inSocket?.destroy()
  agent?.outSocket?.destroy()
  if (connection?._worker) await connection._worker.terminate()
}

export async function terminateTerminal(processHandle: IPty) {
  if (process.platform !== 'win32') {
    try {
      process.kill(-processHandle.pid, 'SIGKILL')
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'ESRCH') throw error
    }
  }
  try {
    processHandle.kill()
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== 'ESRCH' && process.platform !== 'win32')
      throw error
  }
  await releaseTerminalResources(processHandle)
}

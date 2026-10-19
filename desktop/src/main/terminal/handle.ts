import type { IDisposable, IPty } from 'node-pty'
import { releaseTerminalResources, terminateTerminal } from './process'
import { createWindowsJob } from './windowsJob'

export type TerminalExit = { exitCode: number; signal?: number }

export type TerminalHandle = {
  pid: number
  write(data: string): void
  resize(cols: number, rows: number): void
  onData(listener: (data: string) => void): IDisposable
  onExit(listener: (event: TerminalExit) => void): IDisposable
  stop(wasRunning: boolean): Promise<void>
}

export function localTerminalHandle(processHandle: IPty): TerminalHandle {
  let job: ReturnType<typeof createWindowsJob> | undefined
  try {
    job = createWindowsJob(processHandle.pid)
  } catch (error) {
    void terminateTerminal(processHandle).catch(() => {})
    throw error
  }
  return {
    pid: processHandle.pid,
    write: (data) => processHandle.write(data),
    resize: (cols, rows) => processHandle.resize(cols, rows),
    onData: (listener) => processHandle.onData(listener),
    onExit: (listener) => processHandle.onExit(listener),
    stop: async (wasRunning) => {
      try {
        await job?.close()
      } finally {
        if (wasRunning) await terminateTerminal(processHandle)
        else await releaseTerminalResources(processHandle)
      }
    }
  }
}
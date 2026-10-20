import {
  isTerminalReport,
  primaryDeviceAttributesQuery,
  primaryDeviceAttributesReply
} from '../../shared/terminalReports'
import { randomUUID } from 'node:crypto'
import { realpathSync, statSync } from 'node:fs'
import { basename } from 'node:path'
import { spawn, type IDisposable } from 'node-pty'
import type { DesktopEvent } from '../../shared/contracts'
import type {
  TerminalEvent,
  TerminalOptions,
  TerminalOwner,
  TerminalRead,
  TerminalSnapshot
} from '../../shared/terminal'
import { TerminalOutput } from './output'
import { localTerminalHandle, type TerminalHandle } from './handle'
import { remoteTerminalHandle } from './remote'
import type { RemoteConnection } from '../remote/connection'

type Session = {
  terminal: TerminalSnapshot
  process: TerminalHandle
  output: TerminalOutput
  subscriptions: IDisposable[]
  closing?: Promise<void>
  cleanup?: Promise<void>
}

type ManagerOptions = {
  emit: (event: DesktopEvent) => void
  maxSessions?: number
  maxOutputBytes?: number
}

function dimensions(cols = 80, rows = 24) {
  if (!Number.isFinite(cols) || !Number.isFinite(rows))
    throw new Error('Invalid terminal dimensions')
  return {
    cols: Math.min(500, Math.max(20, Math.floor(cols))),
    rows: Math.min(300, Math.max(5, Math.floor(rows)))
  }
}

export class TerminalManager {
  private readonly sessions = new Map<string, Session>()
  private readonly maxSessions: number
  private disposed = false

  constructor(private readonly options: ManagerOptions) {
    this.maxSessions = options.maxSessions ?? 12
    if (!Number.isInteger(this.maxSessions) || this.maxSessions < 1 || this.maxSessions > 50)
      throw new Error('Invalid terminal session limit')
    new TerminalOutput(options.maxOutputBytes)
  }

  create(workspaceId: string, cwd: string, options: TerminalOptions = {}): TerminalSnapshot {
    if (this.disposed) throw new Error('Terminal manager is disposed')
    if (typeof workspaceId !== 'string' || !workspaceId.trim())
      throw new Error('Terminal workspace is required')
    if (this.sessions.size >= this.maxSessions)
      throw new Error(`Terminal session limit reached (${this.maxSessions})`)
    const directory = realpathSync(cwd)
    if (!statSync(directory).isDirectory())
      throw new Error('Terminal working directory must be a folder')
    const size = dimensions(options.cols, options.rows)
    const shell = options.shell ?? (process.platform === 'win32' ? 'powershell.exe' : process.env.SHELL || '/bin/sh')
    const args = options.args ?? (process.platform === 'win32' ? ['-NoLogo', '-NoProfile'] : [])
    if (typeof shell !== 'string' || !shell || shell.includes('\0'))
      throw new Error('Invalid terminal shell')
    if (
      !Array.isArray(args) ||
      args.length > 128 ||
      args.some((arg) => typeof arg !== 'string' || arg.includes('\0'))
    )
      throw new Error('Invalid terminal shell arguments')
    const environment = { ...process.env, TERM: 'xterm-256color', COLORTERM: 'truecolor' }
    const processHandle = spawn(shell, args, {
      ...size,
      cwd: directory,
      env: environment,
      name: 'xterm-256color',
      useConpty: true,
      useConptyDll: true
    })
    return this.attach(workspaceId, directory, shell, size, options.title, localTerminalHandle(processHandle))
  }

  async createRemote(
    workspaceId: string,
    connection: RemoteConnection,
    options: TerminalOptions = {}
  ): Promise<TerminalSnapshot> {
    if (this.disposed) throw new Error('Terminal manager is disposed')
    if (this.sessions.size >= this.maxSessions)
      throw new Error(`Terminal session limit reached (${this.maxSessions})`)
    const size = dimensions(options.cols, options.rows)
    const handle = await remoteTerminalHandle(connection, size.cols, size.rows)
    return this.attach(
      workspaceId,
      connection.root,
      `ssh://${connection.profile.user}@${connection.profile.host}`,
      size,
      options.title ?? connection.profile.name,
      handle
    )
  }

  closeWorkspace(workspaceId: string): Promise<void[]> {
    return Promise.all(this.list(workspaceId).map((terminal) => this.close(terminal.id)))
  }

  private attach(
    workspaceId: string,
    directory: string,
    shell: string,
    size: { cols: number; rows: number },
    title: string | undefined,
    processHandle: TerminalHandle
  ): TerminalSnapshot {
    const terminal: TerminalSnapshot = {
      id: randomUUID(),
      workspaceId,
      cwd: directory,
      shell,
      title:
        typeof title === 'string' && title.trim() ? title.trim().slice(0, 120) : basename(shell),
      pid: processHandle.pid,
      ...size,
      owner: 'user',
      status: 'running',
      createdAt: Date.now(),
      sequence: 0
    }
    const session: Session = {
      terminal,
      process: processHandle,
      output: new TerminalOutput(this.options.maxOutputBytes),
      subscriptions: []
    }
    this.sessions.set(terminal.id, session)
    session.subscriptions.push(
      processHandle.onData((data) => {
        if (terminal.status !== 'running') return
        const queries = data.match(primaryDeviceAttributesQuery)?.length ?? 0
        if (queries) processHandle.write(primaryDeviceAttributesReply.repeat(queries))
        session.output.append(data)
        this.emit(session, { action: 'data', id: terminal.id, data, sequence: ++terminal.sequence })
      })
    )
    session.subscriptions.push(
      processHandle.onExit(({ exitCode, signal }) => {
        if (terminal.status !== 'running') return
        terminal.status = 'exited'
        terminal.exitCode = exitCode
        terminal.signal = signal
        session.cleanup = this.cleanup(session, false)
        void session.cleanup.catch(() => {})
        this.emit(session, {
          action: 'exit',
          id: terminal.id,
          exitCode,
          signal,
          sequence: ++terminal.sequence
        })
        this.state(session)
      })
    )
    return this.state(session)
  }

  list(workspaceId?: string): TerminalSnapshot[] {
    return [...this.sessions.values()]
      .filter(
        (session) => workspaceId === undefined || session.terminal.workspaceId === workspaceId
      )
      .map((session) => ({ ...session.terminal }))
  }

  read(id: string, workspaceId?: string): TerminalRead {
    const session = this.find(id, workspaceId)
    return {
      terminal: { ...session.terminal },
      output: session.output.read(),
      sequence: session.terminal.sequence,
      truncated: session.output.truncated
    }
  }

  write(id: string, data: string, source: TerminalOwner, workspaceId?: string): TerminalSnapshot {
    const session = this.running(id, workspaceId)
    if (source !== 'user' && source !== 'agent') throw new Error('Invalid terminal write source')
    if (session.terminal.owner !== source)
      throw new Error('Terminal ownership must be transferred by the user before writing')
    if (typeof data !== 'string' || Buffer.byteLength(data, 'utf8') > 1024 * 1024)
      throw new Error('Terminal input is too large')
    session.process.write(data)
    return { ...session.terminal }
  }

  report(id: string, data: string, workspaceId?: string): void {
    const session = this.running(id, workspaceId)
    if (!isTerminalReport(data)) throw new Error('Invalid terminal report')
    session.process.write(data)
  }

  resize(id: string, cols: number, rows: number, workspaceId?: string): TerminalSnapshot {
    const session = this.running(id, workspaceId)
    const size = dimensions(cols, rows)
    if (session.terminal.cols === size.cols && session.terminal.rows === size.rows)
      return { ...session.terminal }
    session.process.resize(size.cols, size.rows)
    Object.assign(session.terminal, size)
    return this.state(session)
  }

  setOwner(id: string, owner: TerminalOwner, workspaceId?: string): TerminalSnapshot {
    const session = this.running(id, workspaceId)
    if (owner !== 'user' && owner !== 'agent') throw new Error('Invalid terminal owner')
    session.terminal.owner = owner
    return this.state(session)
  }

  rebindWorkspace(previousId: string, nextId: string): void {
    if (
      typeof previousId !== 'string' ||
      !previousId.trim() ||
      typeof nextId !== 'string' ||
      !nextId.trim()
    )
      throw new Error('Terminal workspace is required')
    if (previousId === nextId) return
    for (const session of this.sessions.values()) {
      if (session.terminal.workspaceId !== previousId) continue
      session.terminal.workspaceId = nextId
      this.state(session)
    }
  }

  async close(id: string, workspaceId?: string): Promise<void> {
    const session = this.find(id, workspaceId)
    if (session.closing) return session.closing
    const wasRunning = session.terminal.status === 'running'
    session.terminal.status = 'closed'
    this.state(session)
    session.closing = (async () => {
      try {
        session.cleanup ??= this.cleanup(session, wasRunning)
        await session.cleanup
      } finally {
        this.release(session)
        this.sessions.delete(id)
      }
    })()
    return session.closing
  }

  async dispose(): Promise<void> {
    this.disposed = true
    await Promise.all([...this.sessions.keys()].map((id) => this.close(id)))
  }

  private find(id: string, workspaceId?: string) {
    const session = this.sessions.get(id)
    if (!session) throw new Error('Terminal not found')
    if (workspaceId !== undefined && session.terminal.workspaceId !== workspaceId)
      throw new Error('Terminal belongs to another workspace')
    return session
  }

  private running(id: string, workspaceId?: string) {
    const session = this.find(id, workspaceId)
    if (session.terminal.status !== 'running')
      throw new Error(`Terminal is ${session.terminal.status}`)
    return session
  }

  private state(session: Session): TerminalSnapshot {
    session.terminal.sequence += 1
    const terminal = { ...session.terminal }
    this.emit(session, { action: 'state', terminal })
    return terminal
  }

  private emit(session: Session, payload: TerminalEvent) {
    this.options.emit({ workspaceId: session.terminal.workspaceId, type: 'terminal', payload })
  }

  private release(session: Session) {
    session.subscriptions.splice(0).forEach((subscription) => subscription.dispose())
  }

  private async cleanup(session: Session, wasRunning: boolean) {
    try {
      this.release(session)
    } finally {
      await session.process.stop(wasRunning)
    }
  }
}

import { randomUUID } from 'node:crypto'
import { z } from 'zod'
import { AtomicStore } from '../persistence/atomicStore'
import type { TerminalManager } from '../terminal/manager'
import type { RemoteConnection } from '../remote/connection'
import type { DesktopEvent } from '../../shared/contracts'
import type { LaunchConfig, LaunchProcess, LaunchRequest, LaunchResponse } from '../../shared/launch'
import { detectLocalUrl, stripAnsi } from './url'
import { powershellLaunchCommand } from './command'
import { createLocalForward, type LocalForward } from '../remote/localForward'

type State = { version: 1; configs: Record<string, LaunchConfig[]> }
type Target = { workspaceId: string; root: string; key: string; remote?: RemoteConnection }
type Options = {
  path: string
  terminals: TerminalManager
  emit(event: DesktopEvent): void
  onUrl?(url: string, workspaceId: string): void
}

const configSchema = z.object({
  id: z.string().uuid().optional(),
  name: z.string().trim().min(1).max(80),
  command: z.string().trim().min(1).max(4000).refine((value) => !/[\r\n\0]/.test(value)),
  url: z.string().trim().max(2000).optional()
})

const requestSchema = z.discriminatedUnion('action', [
  z.object({ action: z.literal('list') }),
  z.object({ action: z.literal('save'), input: configSchema }),
  z.object({ action: z.enum(['remove', 'start', 'stop', 'restart']), id: z.string().uuid() })
])

export class LaunchManager {
  private state: State = { version: 1, configs: {} }
  private store: AtomicStore<State>
  private processes = new Map<
    string,
    LaunchProcess & {
      key: string
      workspaceId: string
      buffer: string
      remote?: RemoteConnection
      forward?: LocalForward
    }
  >()

  constructor(private options: Options) {
    this.store = new AtomicStore(options.path)
  }

  async load() {
    try {
      const saved = (await this.store.read()) as State | undefined
      if (saved?.version === 1 && saved.configs && typeof saved.configs === 'object') this.state = saved
    } catch {}
  }

  private snapshot(key: string): LaunchResponse {
    return structuredClone({
      root: key,
      configs: this.state.configs[key] ?? [],
      processes: [...this.processes.values()]
        .filter((item) => item.key === key)
        .map(({ key: _key, workspaceId: _workspace, buffer: _buffer, remote: _remote, forward: _forward, ...process }) => process)
    })
  }

  private publish(key: string) {
    this.options.emit({ workspaceId: '', type: 'launch', payload: { root: key } })
  }

  private config(key: string, id: string) {
    const config = this.state.configs[key]?.find((item) => item.id === id)
    if (!config) throw new Error('Команда запуска не найдена')
    return config
  }

  async handle(target: Target, input: unknown): Promise<LaunchResponse> {
    const request = requestSchema.parse(input) as LaunchRequest
    const key = target.key
    switch (request.action) {
      case 'save': {
        const config = { ...request.input, id: request.input.id ?? randomUUID() } as LaunchConfig
        if (!config.url) delete config.url
        const configs = (this.state.configs[key] ?? []).filter((item) => item.id !== config.id)
        if (configs.length >= 30) throw new Error('Слишком много команд запуска')
        this.state.configs = { ...this.state.configs, [key]: [...configs, config] }
        await this.store.write(this.state)
        break
      }
      case 'remove':
        await this.stop(request.id)
        this.state.configs = {
          ...this.state.configs,
          [key]: (this.state.configs[key] ?? []).filter((item) => item.id !== request.id)
        }
        await this.store.write(this.state)
        break
      case 'start':
        await this.start(target, this.config(key, request.id))
        break
      case 'stop':
        await this.stop(request.id)
        break
      case 'restart':
        await this.stop(request.id)
        await this.start(target, this.config(key, request.id))
        break
    }
    this.publish(key)
    return this.snapshot(key)
  }

  list(key: string) {
    return this.snapshot(key)
  }

  private async start(target: Target, config: LaunchConfig) {
    const existing = this.processes.get(config.id)
    if (existing?.status === 'running') return
    if (existing) await this.stop(config.id)
    const options = { title: config.name, cols: 120, rows: 30 }
    const terminal = target.remote
      ? await this.options.terminals.createRemote(target.workspaceId, target.remote, options)
      : this.options.terminals.create(target.workspaceId, target.root, options)
    this.processes.set(config.id, {
      configId: config.id,
      terminalId: terminal.id,
      status: 'running',
      startedAt: Date.now(),
      key: target.key,
      workspaceId: target.workspaceId,
      buffer: '',
      remote: target.remote,
      ...(config.url ? { url: config.url } : {})
    })
    const command =
      target.remote || process.platform !== 'win32' ? config.command : powershellLaunchCommand(config.command)
    this.options.terminals.write(terminal.id, command + (target.remote ? '\n' : '\r'), 'user', target.workspaceId)
  }

  private async stop(id: string) {
    const process = this.processes.get(id)
    if (!process) return
    this.processes.delete(id)
    await process.forward?.close()
    await this.options.terminals.close(process.terminalId).catch(() => {})
  }

  terminalEvent(payload: any) {
    const process = [...this.processes.values()].find(
      (item) => item.terminalId === (payload?.id ?? payload?.terminal?.id)
    )
    if (!process) return
    if (payload.action === 'data' && !process.url) {
      process.buffer = (process.buffer + stripAnsi(payload.data)).slice(-4000)
      const url = detectLocalUrl(process.buffer)
      if (url) {
        process.url = url
        void this.expose(process, url)
      }
    }
    if (payload.action === 'exit') {
      process.status = 'exited'
      process.exitCode = payload.exitCode
      this.publish(process.key)
    }
    if (payload.action === 'state' && payload.terminal?.status === 'closed') {
      this.processes.delete(process.configId)
      void process.forward?.close()
      this.publish(process.key)
    }
  }

  private async expose(process: NonNullable<ReturnType<typeof this.processes.get>>, url: string) {
    let visible = url
    if (process.remote) {
      try {
        const parsed = new URL(url)
        process.forward = await createLocalForward(process.remote.client, Number(parsed.port))
        if (!this.processes.has(process.configId)) {
          await process.forward.close()
          return
        }
        parsed.hostname = '127.0.0.1'
        parsed.port = String(process.forward.port)
        visible = parsed.href
      } catch {
        return
      }
    }
    process.url = visible
    this.options.onUrl?.(visible, process.workspaceId)
    this.publish(process.key)
  }

  all() {
    return [...this.processes.values()].map((item) => ({
      configId: item.configId,
      terminalId: item.terminalId,
      status: item.status,
      startedAt: item.startedAt,
      url: item.url,
      exitCode: item.exitCode,
      key: item.key,
      workspaceId: item.workspaceId,
      name: this.state.configs[item.key]?.find((config) => config.id === item.configId)?.name ?? ''
    }))
  }

  async stopProcess(configId: string) {
    const process = this.processes.get(configId)
    await this.stop(configId)
    if (process) this.publish(process.key)
  }

  running(): number {
    return [...this.processes.values()].filter((item) => item.status === 'running').length
  }
}

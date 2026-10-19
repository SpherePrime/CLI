import { randomUUID } from 'node:crypto'
import { posix } from 'node:path'
import type { ClientChannel } from 'ssh2'
import { PrimeTransport } from '../prime/transport'
import type { RemoteConnection } from '../remote/connection'
import { shellQuote } from '../remote/security'
import type { RemoteProfile } from '../../shared/remote'
import { installRemoteRuntime, remoteDirectory } from './assets'

type StartOptions = {
  runtimeDir: string
  stateDirectory?: string
  environment?: Record<string, string>
  progress(message: string, percent?: number): void
  onExit?(code: number | null): void
}

const architectures: Record<string, 'x64' | 'arm64'> = {
  x86_64: 'x64',
  amd64: 'x64',
  aarch64: 'arm64',
  arm64: 'arm64'
}

const environmentName = /^[A-Z_][A-Z0-9_]*$/

function launchScript(binary: string, socket: string, log: string, root: string, environment: Record<string, string>) {
  const exports = Object.entries(environment)
    .filter(([name]) => environmentName.test(name))
    .map(([name, value]) => `export ${name}=${shellQuote(value)}`)
  return [
    'umask 077',
    ...exports,
    `cd ${shellQuote(root)} || exit 90`,
    'exec 3<&0',
    `${shellQuote(binary)} server --host ${shellQuote('unix://' + socket)} </dev/null >${shellQuote(log)} 2>&1 &`,
    'prime=$!',
    '( cat <&3 >/dev/null 2>&1; kill -TERM "$prime" 2>/dev/null ) >/dev/null 2>&1 &',
    'watcher=$!',
    'exec 3<&-',
    'wait "$prime"',
    'code=$?',
    'kill "$watcher" 2>/dev/null',
    `rm -f ${shellQuote(socket)}`,
    'exit "$code"'
  ].join('\n')
}

export class RemoteRuntime {
  private exited = false
  private closing?: Promise<void>
  private constructor(
    private readonly connection: RemoteConnection,
    private readonly channel: ClientChannel,
    readonly socketPath: string,
    readonly transport: PrimeTransport,
    readonly logPath: string
  ) {}

  static async start(profile: RemoteProfile, connection: RemoteConnection, options: StartOptions): Promise<RemoteRuntime> {
    options.progress('remote.preparingRuntime')
    const platform = await connection.exec('uname -sm', { timeout: 15000 })
    const [system, machine] = platform.stdout.trim().split(/\s+/)
    const arch = architectures[machine?.toLowerCase() ?? '']
    if (platform.code || system !== 'Linux' || !arch) throw new Error('remote.unsupportedPlatform')
    const home = await new Promise<string>((resolve, reject) =>
      connection.sftp.realpath('.', (error, path) => (error ? reject(error) : resolve(path)))
    )
    const base = options.stateDirectory ?? posix.join(home, '.prime-desktop')
    let binary: string
    if (profile.runtime === 'existing') {
      if (!profile.runtimePath) throw new Error('remote.runtimeUnavailable')
      binary = profile.runtimePath
    } else {
      binary = await installRemoteRuntime(connection, {
        runtimeDir: options.runtimeDir,
        base,
        arch,
        progress: (_message, percent) => options.progress('remote.uploadingRuntime', percent)
      })
    }
    options.progress('remote.startRuntime')
    const runDirectory = posix.join(base, 'run')
    await remoteDirectory(connection.sftp, runDirectory)
    const identity = randomUUID()
    const socketPath = posix.join(runDirectory, `${identity}.sock`)
    const logPath = posix.join(runDirectory, `${identity}.log`)
    const script = launchScript(binary, socketPath, logPath, connection.root, {
      PRIME_DISABLE_METRICS: '1',
      ...options.environment
    })
    const channel = await new Promise<ClientChannel>((resolve, reject) =>
      connection.client.exec(`sh -c ${shellQuote(script)}`, (error, stream) => (error ? reject(error) : resolve(stream)))
    )
    channel.on('data', () => {})
    channel.stderr.on('data', () => {})
    const transport = new PrimeTransport(socketPath, (callback) =>
      connection.client.openssh_forwardOutStreamLocal(socketPath, (error, stream) =>
        callback(error ? Object.assign(new Error('remote.tunnelDenied'), { cause: error }) : null, stream)
      )
    )
    const runtime = new RemoteRuntime(connection, channel, socketPath, transport, logPath)
    channel.once('close', (code: number | null) => {
      runtime.exited = true
      if (!runtime.closing) options.onExit?.(code)
    })
    try {
      await runtime.waitReady()
      return runtime
    } catch (error) {
      await runtime.close()
      throw error
    }
  }

  private async waitReady() {
    const started = Date.now()
    let failure: Error | undefined
    while (Date.now() - started < 60000) {
      if (this.exited) throw new Error('remote.runtimeFailed')
      try {
        await this.transport.request('GET', '/v1/health', undefined, 2000)
        const capabilities: any = await this.transport.request('GET', '/v1/capabilities', undefined, 5000)
        if (
          capabilities.protocol_version !== 1 ||
          !capabilities.pending_requests ||
          !capabilities.run_registry ||
          !capabilities.queue_control
        )
          throw new Error('remote.runtimeUnavailable')
        return
      } catch (error) {
        failure = error as Error
        if (failure.message === 'remote.runtimeUnavailable') throw failure
        await new Promise((resolve) => setTimeout(resolve, 250))
      }
    }
    if (failure?.message === 'remote.tunnelDenied') throw failure
    throw new Error('remote.runtimeFailed')
  }

  get running() {
    return !this.exited
  }

  close(): Promise<void> {
    this.closing ??= (async () => {
      if (!this.exited) {
        await this.transport.request('POST', '/v1/control', { command: 'shutdown_if_idle' }, 2000).catch(() => {})
        const closed = new Promise<void>((resolve) => {
          if (this.exited) resolve()
          else this.channel.once('close', () => resolve())
        })
        this.channel.end()
        await Promise.race([closed, new Promise((resolve) => setTimeout(resolve, 8000))])
        if (!this.exited) this.channel.close()
      }
      await this.connection
        .exec(`rm -f ${shellQuote(this.socketPath)} ${shellQuote(this.logPath)}`, { timeout: 5000 })
        .catch(() => {})
    })()
    return this.closing
  }
}

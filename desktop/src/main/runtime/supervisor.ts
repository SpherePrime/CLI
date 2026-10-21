import { spawn, type ChildProcess } from 'node:child_process'
import { existsSync } from 'node:fs'
import { createHash, randomUUID } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { PrimeTransport } from '../prime/transport'

export class RuntimeSupervisor {
  process?: ChildProcess
  transport?: PrimeTransport
  status = 'starting'
  version: any = null
  lastError = ''
  private shuttingDown = false
  constructor(
    private runtimeDir: string,
    private emit: (event: any) => void
  ) {}
  async start(): Promise<void> {
    const binary =
      process.env.PRIME_DESKTOP_RUNTIME ||
      join(this.runtimeDir, process.platform === 'win32' ? 'prime.exe' : 'prime')
    if (!existsSync(binary)) throw new Error(`Не найден встроенный Prime: ${binary}`)
    if (!process.env.PRIME_DESKTOP_RUNTIME) {
      const manifest = JSON.parse(await readFile(join(this.runtimeDir, 'manifest.json'), 'utf8'))
      if (
        manifest.platform !== process.platform ||
        manifest.arch !== process.arch ||
        manifest.protocolVersion !== 1
      )
        throw new Error('Встроенный Prime несовместим с платформой или протоколом Desktop')
      const checksum = createHash('sha256')
        .update(await readFile(binary))
        .digest('hex')
      if (checksum !== manifest.sha256)
        throw new Error('Контрольная сумма Prime не совпадает с manifest')
    }
    const identity = randomUUID()
    const endpoint =
      process.platform === 'win32'
        ? `\\\\.\\pipe\\prime-desktop-${identity}`
        : join(process.env.XDG_RUNTIME_DIR || '/tmp', `prime-desktop-${identity}.sock`)
    this.transport = new PrimeTransport(endpoint)
    const args = [
      'server',
      '--host',
      `${process.platform === 'win32' ? 'npipe' : 'unix'}://${endpoint.replaceAll('\\', '/')}`
    ]
    this.process = spawn(binary, args, {
      windowsHide: true,
      shell: false,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: {
        ...process.env,
        PRIME_DESKTOP: '1',
        PRIME_DISABLE_METRICS: '1',
        PRIME_PARENT_PID: String(process.pid)
      }
    })
    this.process.stdout?.on('data', () => {})
    this.process.stderr?.on('data', (chunk: Buffer) => {
      this.lastError = chunk.toString('utf8').slice(-2000)
    })
    this.process.on('error', (error) => {
      this.status = 'failed'
      this.lastError = error.message
      this.emit({ type: 'runtime', payload: { status: this.status, error: error.message } })
    })
    this.process.on('exit', (code) => {
      if (!this.shuttingDown) {
        this.status = 'failed'
        this.emit({
          type: 'runtime',
          payload: { status: 'failed', error: `Prime завершился (${code}). ${this.lastError}` }
        })
      }
    })
    const start = Date.now()
    while (Date.now() - start < 20000) {
      if (this.status === 'failed') throw new Error(this.lastError)
      try {
        await this.transport.request('GET', '/v1/health', undefined, 1000)
        this.version = await this.transport.request('GET', '/v1/version')
        const capabilities: any = await this.transport.request('GET', '/v1/capabilities')
        if (
          capabilities.protocol_version !== 1 ||
          !capabilities.pending_requests ||
          !capabilities.run_registry ||
          !capabilities.queue_control
        ) {
          this.process.kill()
          this.status = 'failed'
          throw new Error('Версия Prime не поддерживает Desktop API')
        }
        this.version.capabilities = capabilities
        this.status = 'ready'
        return
      } catch (error) {
        if (this.status === 'failed') throw error
        await new Promise((resolve) => setTimeout(resolve, 200))
      }
    }
    this.process.kill()
    this.status = 'failed'
    throw new Error(`Prime не запустился за 20 секунд. ${this.lastError}`)
  }
  async stop(): Promise<void> {
    this.shuttingDown = true
    try {
      await this.transport?.request('POST', '/v1/control', { command: 'shutdown_if_idle' }, 3000)
    } catch {}
    if (this.process && this.process.exitCode === null) this.process.kill()
  }
}

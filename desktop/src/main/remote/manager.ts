import { createHash } from 'node:crypto'
import { open } from 'node:fs/promises'
import { Client, type ConnectConfig } from 'ssh2'
import { AtomicStore } from '../persistence/atomicStore'
import type {
  RemoteCredentials,
  RemoteProfile,
  RemoteProfileInput,
  RemoteProfileState,
  RemoteRequest,
  RemoteSnapshot
} from '../../shared/remote'
import { validateProfile } from './security'
import { RemoteConnection } from './connection'
import type { RemoteCredentialStore } from './credentials'
type State = { version: 1; profiles: RemoteProfile[]; hosts: Record<string, string> }
type Options = {
  path: string
  credentials?: RemoteCredentialStore
  emit(snapshot: RemoteSnapshot): void
  onReady?(profile: RemoteProfile, connection: RemoteConnection): Promise<void>
  onDisconnect?(id: string): Promise<void>
  onLost?(id: string): void
}
const hostId = (profile: RemoteProfile) => `[${profile.host.toLowerCase()}]:${profile.port}`
export class RemoteManager {
  private state: State = { version: 1, profiles: [], hosts: {} }
  private store: AtomicStore<State>
  private error?: string
  private credentialError?: string
  private loaded = false
  private queue: Promise<unknown> = Promise.resolve()
  private statuses = new Map<string, Partial<RemoteProfileState>>()
  private clients = new Map<string, Client>()
  private connections = new Map<string, RemoteConnection>()
  private generations = new Map<string, number>()
  private pending = new Map<string, Promise<unknown>>()
  constructor(private options: Options) {
    this.store = new AtomicStore(options.path)
  }
  async load() {
    await this.options.credentials?.load().catch(() => {
      this.credentialError = 'remote.credentialsCorrupt'
    })
    try {
      const saved = (await this.store.read()) as State | undefined
      if (saved) {
        if (
          saved.version !== 1 ||
          !Array.isArray(saved.profiles) ||
          !saved.hosts ||
          typeof saved.hosts !== 'object' ||
          Array.isArray(saved.hosts)
        )
          throw new Error()
        const profiles = saved.profiles.map(validateProfile)
        if (
          new Set(profiles.map((profile) => profile.id)).size !== profiles.length ||
          Object.values(saved.hosts).some(
            (value) => typeof value !== 'string' || !/^SHA256:[A-Za-z0-9+/]{43}$/.test(value)
          )
        )
          throw new Error()
        this.state = { version: 1, profiles, hosts: { ...saved.hosts } }
      }
    } catch {
      this.error = 'remote.corrupt'
    }
    this.loaded = true
    return this.list()
  }
  list(): RemoteSnapshot {
    return structuredClone({
      profiles: this.state.profiles.map((profile) => ({
        ...profile,
        ...(this.options.credentials
          ? { hasSavedCredentials: this.options.credentials.has(profile) }
          : {}),
        status: 'disconnected',
        ...this.statuses.get(profile.id)
      })),
      ...(this.error ? { error: this.error } : {}),
      ...(this.options.credentials
        ? {
            credentialStorageAvailable: this.options.credentials.available && !this.credentialError
          }
        : {}),
      ...(this.credentialError ? { credentialError: this.credentialError } : {})
    })
  }
  private emit() {
    this.options.emit(this.list())
  }
  private writable() {
    if (!this.loaded) throw new Error('remote.notLoaded')
    if (this.error) throw new Error(this.error)
  }
  private serialize<T>(operation: () => Promise<T>): Promise<T> {
    const result = this.queue.then(operation)
    this.queue = result.catch(() => {})
    return result
  }
  private profile(id: string) {
    const profile = this.state.profiles.find((item) => item.id === id)
    if (!profile) throw new Error('remote.profileMissing')
    return profile
  }
  async save(input: RemoteProfileInput) {
    return this.serialize(async () => {
      this.writable()
      const profile = validateProfile(input)
      if (this.clients.has(profile.id) || this.pending.has(profile.id))
        throw new Error('remote.disconnectFirst')
      const next = {
        ...this.state,
        profiles: [...this.state.profiles.filter((item) => item.id !== profile.id), profile]
      }
      await this.store.write(next)
      this.state = next
      if (
        this.options.credentials &&
        !this.credentialError &&
        !this.options.credentials.has(profile)
      )
        await this.options.credentials.remove(profile.id)
      this.statuses.delete(profile.id)
      this.emit()
      return this.list()
    })
  }
  async remove(id: string) {
    await this.disconnect(id)
    return this.serialize(async () => {
      this.writable()
      await this.options.credentials?.remove(id)
      const next = { ...this.state, profiles: this.state.profiles.filter((item) => item.id !== id) }
      await this.store.write(next)
      this.state = next
      this.statuses.delete(id)
      this.emit()
      return this.list()
    })
  }
  async trust(id: string, fingerprint: string) {
    return this.serialize(async () => {
      this.writable()
      const profile = this.profile(id)
      const challenge = this.statuses.get(id)?.challenge
      if (!challenge || challenge.fingerprint !== fingerprint)
        throw new Error('remote.staleChallenge')
      const next = { ...this.state, hosts: { ...this.state.hosts, [hostId(profile)]: fingerprint } }
      await this.store.write(next)
      this.state = next
      this.statuses.set(id, { status: 'disconnected' })
      this.emit()
      return this.list()
    })
  }
  connection(id: string) {
    const connection = this.connections.get(id)
    if (!connection) throw new Error('remote.notConnected')
    return connection
  }
  setProgress(id: string, message: string, percent?: number) {
    const status = this.statuses.get(id)
    if (status?.status === 'preparing') {
      this.statuses.set(id, { ...status, progress: { message, percent } })
      this.emit()
    }
  }
  async connect(
    id: string,
    credentials: RemoteCredentials = {},
    options: { prepareWorkspace?: boolean } = {}
  ) {
    this.writable()
    const profile = this.profile(id)
    if (this.pending.has(id) || this.clients.has(id)) throw new Error('remote.alreadyConnecting')
    const generation = (this.generations.get(id) ?? 0) + 1
    this.generations.set(id, generation)
    this.statuses.set(id, { status: 'connecting' })
    this.emit()
    const operation = this.establish(
      profile,
      credentials,
      generation,
      options.prepareWorkspace !== false
    )
    this.pending.set(id, operation)
    try {
      await operation
    } finally {
      if (this.pending.get(id) === operation) this.pending.delete(id)
    }
    return this.list()
  }
  private async establish(
    profile: RemoteProfile,
    credentials: RemoteCredentials,
    generation: number,
    prepareWorkspace: boolean
  ) {
    const id = profile.id
    const client = new Client()
    this.clients.set(id, client)
    const current = () => this.generations.get(id) === generation
    let challenged = false
    let intentional = false
    let ready = false
    const lost = () => {
      if (!current() || intentional || !ready) return
      this.generations.set(id, generation + 1)
      this.connections.delete(id)
      this.clients.delete(id)
      this.statuses.set(id, { status: 'error', error: 'remote.connectionLost' })
      this.options.onLost?.(id)
      this.emit()
    }
    client.on('close', lost)
    client.on('error', lost)
    try {
      const supplied = Boolean(credentials.password || credentials.passphrase)
      if (this.options.credentials && !this.credentialError) {
        if (supplied && this.options.credentials.available)
          await this.options.credentials.save(profile, credentials)
        if (!supplied) credentials = this.options.credentials.get(profile) ?? credentials
      }
      if (!current()) return
      const config: ConnectConfig = {
        host: profile.host,
        port: profile.port,
        username: profile.user,
        readyTimeout: 15000,
        keepaliveInterval: 10000,
        keepaliveCountMax: 3,
        hostVerifier: (key: Buffer) => {
          if (!current()) return false
          const fingerprint =
            'SHA256:' + createHash('sha256').update(key).digest('base64').replace(/=+$/, '')
          const previous = this.state.hosts[hostId(profile)]
          if (previous === fingerprint) return true
          challenged = true
          this.statuses.set(id, {
            status: 'verifying',
            challenge: { fingerprint, ...(previous ? { previous } : {}) }
          })
          this.emit()
          return false
        }
      }
      if (profile.auth === 'password') {
        if (!credentials.password) throw new Error('remote.credentialsRequired')
        config.password = credentials.password
      }
      if (profile.auth === 'agent')
        config.agent =
          process.env.SSH_AUTH_SOCK ||
          (process.platform === 'win32' ? '\\\\.\\pipe\\openssh-ssh-agent' : undefined)
      if (profile.auth === 'key') {
        const file = await open(profile.keyPath!, 'r')
        try {
          const info = await file.stat()
          if (!info.isFile() || info.size > 1024 * 1024) throw new Error('remote.invalidKey')
          const buffer = Buffer.alloc(1024 * 1024 + 1)
          const { bytesRead } = await file.read(buffer)
          if (bytesRead > 1024 * 1024) throw new Error('remote.invalidKey')
          config.privateKey = buffer.subarray(0, bytesRead)
          config.passphrase = credentials.passphrase
        } finally {
          await file.close()
        }
      }
      if (!current()) return
      await new Promise<void>((resolve, reject) => {
        client.once('ready', resolve)
        client.once('error', reject)
        client.once('close', () => reject(new Error('remote.connectionClosed')))
        client.connect(config)
      })
      if (!current()) return
      ready = true
      const sftp = await new Promise<import('ssh2').SFTPWrapper>((resolve, reject) =>
        client.sftp((error, value) => (error ? reject(error) : resolve(value)))
      )
      const connection = await new RemoteConnection(client, sftp, profile).initialize()
      if (!current()) {
        connection.close()
        return
      }
      this.connections.set(id, connection)
      this.statuses.set(id, { status: 'preparing' })
      this.emit()
      if (prepareWorkspace) await this.options.onReady?.(profile, connection)
      if (!current()) return
      this.statuses.set(id, { status: 'connected' })
      this.emit()
    } catch (error) {
      intentional = true
      if (current()) {
        await this.options.onDisconnect?.(id).catch(() => {})
        this.connections.delete(id)
        this.clients.delete(id)
        if (!challenged) {
          const code = (error as Error).message
          const safe = /^remote\.[a-zA-Z]+$/.test(code) ? code : 'remote.connectionFailed'
          this.statuses.set(id, { status: 'error', error: safe })
        }
        this.emit()
      }
      client.destroy()
    } finally {
      if (!current() && this.statuses.get(id)?.status !== 'disconnecting') {
        await this.options.onDisconnect?.(id).catch(() => {})
        client.destroy()
      }
    }
  }
  async disconnect(id: string) {
    this.generations.set(id, (this.generations.get(id) ?? 0) + 1)
    this.statuses.set(id, { status: 'disconnecting' })
    this.emit()
    try {
      await this.options.onDisconnect?.(id)
      if (this.pending.has(id)) {
        await this.pending.get(id)?.catch(() => {})
        await this.options.onDisconnect?.(id)
      }
    } finally {
      this.connections.get(id)?.close()
      this.clients.get(id)?.destroy()
      this.connections.delete(id)
      this.clients.delete(id)
      await this.pending.get(id)?.catch(() => {})
      this.statuses.set(id, { status: 'disconnected' })
      this.emit()
    }
    return this.list()
  }
  async fail(id: string, code = 'remote.runtimeFailed') {
    const disconnecting = this.disconnect(id)
    const generation = this.generations.get(id)
    await disconnecting
    if (this.generations.get(id) === generation && !this.clients.has(id) && !this.pending.has(id)) {
      this.statuses.set(id, {
        status: 'error',
        error: /^remote\.[a-zA-Z]+$/.test(code) ? code : 'remote.runtimeFailed'
      })
      this.emit()
    }
    return this.list()
  }
  async shutdown() {
    await Promise.all(this.state.profiles.map((profile) => this.disconnect(profile.id)))
  }
  async handle(request: RemoteRequest) {
    switch (request.action) {
      case 'list':
        return this.list()
      case 'save':
        return this.save(request.input)
      case 'remove':
        return this.remove(request.id)
      case 'connect':
        return this.connect(request.id, request.credentials)
      case 'forgetCredentials':
        this.writable()
        this.profile(request.id)
        await this.options.credentials?.remove(request.id)
        this.emit()
        return this.list()
      case 'trust':
        return this.trust(request.id, request.fingerprint)
      case 'disconnect':
        return this.disconnect(request.id)
    }
  }
}

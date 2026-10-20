import { createHash } from 'node:crypto'
import { AtomicStore } from '../persistence/atomicStore'
import type { RemoteCredentials, RemoteProfile } from '../../shared/remote'

export type CredentialCipher = {
  available(): boolean
  encrypt(text: string): Buffer
  decrypt(bytes: Buffer): string
}
type Entry = { binding: string; encrypted: string }
type State = { version: 1; entries: Record<string, Entry> }

function binding(profile: RemoteProfile) {
  return createHash('sha256')
    .update(
      JSON.stringify([
        profile.host.toLowerCase(),
        profile.port,
        profile.user,
        profile.auth,
        profile.keyPath ?? ''
      ])
    )
    .digest('hex')
}

export class RemoteCredentialStore {
  private state: State = { version: 1, entries: {} }
  private readonly store: AtomicStore<State>
  private queue: Promise<unknown> = Promise.resolve()
  private loaded = false
  constructor(
    path: string,
    private readonly cipher: CredentialCipher
  ) {
    this.store = new AtomicStore(path)
  }
  get available() {
    return this.cipher.available()
  }
  async load() {
    try {
      const saved = (await this.store.read()) as State | undefined
      if (saved) {
        if (
          saved.version !== 1 ||
          !saved.entries ||
          typeof saved.entries !== 'object' ||
          Array.isArray(saved.entries)
        )
          throw new Error()
        for (const entry of Object.values(saved.entries)) {
          if (
            !entry ||
            !/^[a-f0-9]{64}$/.test(entry.binding) ||
            typeof entry.encrypted !== 'string' ||
            entry.encrypted.length > 65536
          )
            throw new Error()
        }
        this.state = saved
      }
      this.loaded = true
    } catch {
      throw new Error('remote.credentialsCorrupt')
    }
  }
  has(profile: RemoteProfile) {
    return Boolean(this.state.entries[profile.id]?.binding === binding(profile))
  }
  get(profile: RemoteProfile): RemoteCredentials | undefined {
    if (!this.available || !this.has(profile)) return undefined
    try {
      const payload = JSON.parse(
        this.cipher.decrypt(Buffer.from(this.state.entries[profile.id].encrypted, 'base64'))
      )
      if (payload?.binding !== binding(profile)) throw new Error()
      const credentials = payload.credentials
      if (!credentials || typeof credentials !== 'object' || Array.isArray(credentials))
        throw new Error()
      for (const value of Object.values(credentials)) {
        if (typeof value !== 'string' || value.length > 4096) throw new Error()
      }
      return { password: credentials.password, passphrase: credentials.passphrase }
    } catch {
      throw new Error('remote.credentialsUnreadable')
    }
  }
  private update(change: (next: State) => void) {
    const operation = this.queue
      .catch(() => {})
      .then(async () => {
        if (!this.loaded) throw new Error('remote.credentialsCorrupt')
        const next: State = { version: 1, entries: { ...this.state.entries } }
        change(next)
        await this.store.write(next)
        this.state = next
      })
    this.queue = operation
    return operation
  }
  save(profile: RemoteProfile, credentials: RemoteCredentials) {
    return this.update((next) => {
      if (!this.available) throw new Error('remote.secureStorageUnavailable')
      try {
        next.entries[profile.id] = {
          binding: binding(profile),
          encrypted: this.cipher
            .encrypt(
              JSON.stringify({
                binding: binding(profile),
                credentials: { password: credentials.password, passphrase: credentials.passphrase }
              })
            )
            .toString('base64')
        }
      } catch {
        throw new Error('remote.secureStorageUnavailable')
      }
    })
  }
  remove(id: string) {
    return this.update((next) => {
      delete next.entries[id]
    })
  }
}

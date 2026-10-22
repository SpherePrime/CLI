import { safeStorage } from 'electron'
import { AtomicStore } from '../persistence/atomicStore'

type State = { version: 1; encrypted: string }

export class SudoSecretStore {
  private readonly store: AtomicStore<State>
  private state?: State
  private loaded = false
  constructor(
    path: string,
    private readonly supported = process.platform === 'linux'
  ) {
    this.store = new AtomicStore(path)
  }
  get available() {
    return (
      this.supported &&
      safeStorage.isEncryptionAvailable() &&
      safeStorage.getSelectedStorageBackend() !== 'basic_text'
    )
  }
  async load() {
    try {
      const saved = (await this.store.read()) as State | undefined
      if (
        saved &&
        saved.version === 1 &&
        typeof saved.encrypted === 'string' &&
        saved.encrypted.length <= 65536
      )
        this.state = saved
      this.loaded = true
    } catch {
      this.loaded = true
    }
  }
  has() {
    return Boolean(this.state?.encrypted)
  }
  get(): string | undefined {
    if (!this.loaded || !this.state || !this.available) return undefined
    try {
      const password = safeStorage.decryptString(Buffer.from(this.state.encrypted, 'base64'))
      return password || undefined
    } catch {
      return undefined
    }
  }
  async set(password: string) {
    if (!this.available) throw new Error('secure storage unavailable')
    const encrypted = safeStorage.encryptString(password).toString('base64')
    const next: State = { version: 1, encrypted }
    await this.store.write(next)
    this.state = next
    this.loaded = true
  }
  async remove() {
    await this.store.write({ version: 1, encrypted: '' })
    this.state = undefined
    this.loaded = true
  }
}

export type SudoSecretRequest = { action: 'status' } | { action: 'set'; password: string } | { action: 'remove' }
export type SudoSecretResponse = { hasPassword: boolean; storageAvailable: boolean }

export function sudoSecretRequest(store: SudoSecretStore, input: unknown) {
  const request = input as SudoSecretRequest
  if (request?.action === 'set') {
    if (typeof request.password !== 'string' || !request.password || request.password.length > 4096)
      throw new Error('invalid password')
    return store.set(request.password).then(() => status(store))
  }
  if (request?.action === 'remove') return store.remove().then(() => status(store))
  if (request?.action === 'status') return Promise.resolve(status(store))
  throw new Error('invalid request')
}

function status(store: SudoSecretStore): SudoSecretResponse {
  return { hasPassword: store.has(), storageAvailable: store.available }
}

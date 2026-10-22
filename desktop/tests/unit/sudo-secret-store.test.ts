import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mkdtempSync, readFileSync, rmSync, existsSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { SudoSecretStore, sudoSecretRequest } from '../../src/main/sudo/secretStore'

const safeStorage = vi.hoisted(() => ({
  isEncryptionAvailable: () => encryptionAvailable,
  getSelectedStorageBackend: () => storageBackend,
  encryptString: (text: string) => Buffer.from(`enc:${text}`, 'utf8'),
  decryptString: (bytes: Buffer) =>
    Buffer.from(bytes.toString('utf8'), 'utf8').subarray(4).toString()
}))

let encryptionAvailable = true
let storageBackend = 'keyring'

vi.mock('electron', () => ({ safeStorage }))

function createStore(path: string, supported = true) {
  return new SudoSecretStore(path, supported)
}

describe('SudoSecretStore', () => {
  let directory: string
  let path: string
  beforeEach(() => {
    directory = mkdtempSync(join(tmpdir(), 'prime-sudo-'))
    path = join(directory, 'sudo-credentials.json')
    encryptionAvailable = true
    storageBackend = 'keyring'
  })
  afterEach(() => {
    rmSync(directory, { recursive: true, force: true })
  })

  it('stores nothing until set', async () => {
    const store = createStore(path)
    await store.load()
    expect(store.has()).toBe(false)
    expect(store.get()).toBeUndefined()
  })

  it('encrypts the password and never writes plaintext', async () => {
    const store = createStore(path)
    await store.load()
    await store.set('hunter2')

    const raw = readFileSync(path, 'utf8')
    expect(raw).not.toContain('hunter2')
    expect(Buffer.from(JSON.parse(raw).encrypted, 'base64').toString()).toBe('enc:hunter2')
    expect(store.has()).toBe(true)
    expect(store.get()).toBe('hunter2')
  })

  it('reads the password back after reload', async () => {
    const first = createStore(path)
    await first.load()
    await first.set('secret')

    const second = createStore(path)
    await second.load()
    expect(second.has()).toBe(true)
    expect(second.get()).toBe('secret')
  })

  it('removes the password', async () => {
    const store = createStore(path)
    await store.load()
    await store.set('secret')
    await store.remove()
    expect(store.has()).toBe(false)
    expect(store.get()).toBeUndefined()
    expect(existsSync(path)).toBe(true)
  })

  it('refuses plaintext storage backends', async () => {
    storageBackend = 'basic_text'
    const store = createStore(path)
    await store.load()
    expect(store.available).toBe(false)
    await expect(store.set('secret')).rejects.toThrow()
    expect(store.has()).toBe(false)
  })

  it('returns nothing when encryption is unavailable', async () => {
    encryptionAvailable = false
    const store = createStore(path)
    await store.load()
    await expect(store.set('secret')).rejects.toThrow()
    expect(store.has()).toBe(false)
    expect(store.get()).toBeUndefined()
  })

  it('does not expose the password outside Linux builds', async () => {
    const store = createStore(path, false)
    await store.load()
    expect(store.available).toBe(false)
    await expect(store.set('secret')).rejects.toThrow()
  })
})

describe('sudoSecretRequest', () => {
  let directory: string
  beforeEach(() => {
    directory = mkdtempSync(join(tmpdir(), 'prime-sudo-req-'))
  })
  afterEach(() => {
    rmSync(directory, { recursive: true, force: true })
  })

  it('sets and reports the stored password', async () => {
    const store = new SudoSecretStore(join(directory, 'sudo.json'), true)
    await store.load()

    expect(await sudoSecretRequest(store, { action: 'status' })).toEqual({
      hasPassword: false,
      storageAvailable: true
    })
    expect(await sudoSecretRequest(store, { action: 'set', password: 'root-pass' })).toEqual({
      hasPassword: true,
      storageAvailable: true
    })
  })

  it('removes the password', async () => {
    const store = new SudoSecretStore(join(directory, 'sudo.json'), true)
    await store.load()
    await sudoSecretRequest(store, { action: 'set', password: 'root-pass' })

    expect(await sudoSecretRequest(store, { action: 'remove' })).toEqual({
      hasPassword: false,
      storageAvailable: true
    })
  })

  it('rejects invalid passwords and unknown actions', async () => {
    const store = new SudoSecretStore(join(directory, 'sudo.json'), true)
    await store.load()

    expect(() => sudoSecretRequest(store, { action: 'set', password: '' })).toThrow()
    expect(() =>
      sudoSecretRequest(store, { action: 'set', password: 'x'.repeat(5000) })
    ).toThrow()
    expect(() => sudoSecretRequest(store, { action: 'bogus' })).toThrow()
  })
})

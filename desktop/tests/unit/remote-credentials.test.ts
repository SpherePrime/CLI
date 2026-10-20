import { expect, it } from 'vitest'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto'
import { RemoteCredentialStore } from '../../src/main/remote/credentials'
import type { RemoteProfile } from '../../src/shared/remote'

it('restores encrypted credentials after restart without exposing them in profile metadata and binds them to the server', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'prime-secrets-'))
  const key = randomBytes(32)
  const cipher = {
    available: () => true,
    encrypt: (text: string) => {
      const nonce = randomBytes(12)
      const encryptor = createCipheriv('aes-256-gcm', key, nonce)
      const body = Buffer.concat([encryptor.update(text, 'utf8'), encryptor.final()])
      return Buffer.concat([nonce, encryptor.getAuthTag(), body])
    },
    decrypt: (bytes: Buffer) => {
      const decryptor = createDecipheriv('aes-256-gcm', key, bytes.subarray(0, 12))
      decryptor.setAuthTag(bytes.subarray(12, 28))
      return Buffer.concat([decryptor.update(bytes.subarray(28)), decryptor.final()]).toString(
        'utf8'
      )
    }
  }
  const profile: RemoteProfile = {
    id: 'a',
    name: 'Server',
    host: 'example.test',
    port: 22,
    user: 'dev',
    root: '/project',
    auth: 'password',
    runtime: 'bundled'
  }
  try {
    const path = join(directory, 'credentials.json')
    const first = new RemoteCredentialStore(path, cipher)
    await first.load()
    await first.save(profile, { password: 'private-password', passphrase: 'private-key-password' })
    expect(await readFile(path, 'utf8')).not.toContain('private-password')
    const restored = new RemoteCredentialStore(path, cipher)
    await restored.load()
    expect(restored.has(profile)).toBe(true)
    expect(restored.get(profile)).toEqual({
      password: 'private-password',
      passphrase: 'private-key-password'
    })
    expect(restored.get({ ...profile, host: 'other.test' })).toBeUndefined()
    const unavailable = new RemoteCredentialStore(path, { ...cipher, available: () => false })
    await unavailable.load()
    await expect(unavailable.save(profile, { password: 'new-password' })).rejects.toThrow(
      'remote.secureStorageUnavailable'
    )
    expect(await readFile(path, 'utf8')).not.toContain('new-password')
    await restored.remove(profile.id)
    expect(restored.has(profile)).toBe(false)
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})

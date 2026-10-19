import { afterEach, describe, expect, it } from 'vitest'
import { generateKeyPairSync } from 'node:crypto'
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { Server } from 'ssh2'
import { RemoteManager } from '../../src/main/remote/manager'
const key = generateKeyPairSync('rsa', { modulusLength: 2048 }).privateKey.export({
  type: 'pkcs1',
  format: 'pem'
})
const cleanup: (() => Promise<unknown>)[] = []
afterEach(async () => {
  for (const close of cleanup.splice(0).reverse()) await close()
})
async function fixture() {
  let authentications = 0
  const clients: import('ssh2').Connection[] = []
  const server = new Server({ hostKeys: [key] }, (client) => {
    clients.push(client)
    client.on('authentication', (context) => {
      authentications++
      context.reject()
    })
    client.on('error', () => {})
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  cleanup.push(async () => {
    clients.forEach((client) => client.end())
    await new Promise<void>((resolve) => server.close(() => resolve()))
  })
  const directory = await mkdtemp(join(tmpdir(), 'prime-remote-'))
  cleanup.push(() => rm(directory, { recursive: true, force: true }))
  const path = join(directory, 'remote.json')
  const manager = new RemoteManager({ path, emit() {} })
  await manager.load()
  const snapshot = await manager.save({
    name: 'Linux',
    host: '127.0.0.1',
    port: (server.address() as import('node:net').AddressInfo).port,
    user: 'dev',
    root: '/project',
    auth: 'password',
    runtime: 'bundled'
  })
  cleanup.push(() => manager.shutdown())
  return { manager, path, id: snapshot.profiles[0].id, authentications: () => authentications }
}
describe('remote SSH trust', () => {
  it('warns on changed pinned keys and rejects stale trust attempts', async () => {
    const f = await fixture()
    await f.manager.connect(f.id, { password: 'private-password' })
    const fingerprint = f.manager.list().profiles[0].challenge!.fingerprint
    await f.manager.trust(f.id, fingerprint)
    const saved = JSON.parse(await readFile(f.path, 'utf8'))
    const host = Object.keys(saved.hosts)[0]
    const previous = 'SHA256:' + 'A'.repeat(43)
    saved.hosts[host] = previous
    await writeFile(f.path, JSON.stringify(saved))
    const manager = new RemoteManager({ path: f.path, emit() {} })
    cleanup.push(() => manager.shutdown())
    await manager.load()
    await manager.connect(f.id, { password: 'private-password' })
    expect(manager.list().profiles[0].challenge).toEqual({ fingerprint, previous })
    await expect(manager.trust(f.id, previous)).rejects.toThrow('remote.staleChallenge')
    expect(f.authentications()).toBe(0)
  })
  it('rejects unknown keys before authentication and never persists credentials', async () => {
    const f = await fixture()
    await f.manager.connect(f.id, { password: 'private-password' })
    const profile = f.manager.list().profiles[0]
    expect(profile.status).toBe('verifying')
    expect(profile.challenge?.fingerprint).toMatch(/^SHA256:/)
    expect(f.authentications()).toBe(0)
    expect(await readFile(f.path, 'utf8')).not.toContain('private-password')
    await expect(f.manager.trust(f.id, 'SHA256:wrong')).rejects.toThrow('remote.staleChallenge')
    await f.manager.trust(f.id, profile.challenge!.fingerprint)
    await f.manager.connect(f.id, { password: 'private-password' })
    expect(f.authentications()).toBeGreaterThan(0)
    expect(JSON.stringify(f.manager.list())).not.toContain('private-password')
    expect(f.manager.list().profiles[0].error).toBe('remote.connectionFailed')
  })
  it('keeps corrupt stores read only without replacing their bytes', async () => {
    const f = await fixture()
    await f.manager.shutdown()
    await writeFile(f.path, 'broken')
    const manager = new RemoteManager({ path: f.path, emit() {} })
    await manager.load()
    expect(manager.list().error).toBe('remote.corrupt')
    await expect(manager.remove(f.id)).rejects.toThrow('remote.corrupt')
    expect(await readFile(f.path, 'utf8')).toBe('broken')
  })
  it('disconnect during connect cannot resurrect a connection', async () => {
    const f = await fixture()
    const attempt = f.manager.connect(f.id, { password: 'private-password' })
    await f.manager.disconnect(f.id)
    await attempt
    expect(f.manager.list().profiles[0].status).toBe('disconnected')
    expect(() => f.manager.connection(f.id)).toThrow('remote.notConnected')
  })
})

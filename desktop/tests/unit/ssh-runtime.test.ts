import { expect, it } from 'vitest'
import { join, resolve } from 'node:path'
import { createSshLinuxFixture, detectSshLinuxFixture } from '../fixtures/ssh-linux'
import { RemoteManager } from '../../src/main/remote/manager'
import { RemoteRuntime } from '../../src/main/remoteRuntime/session'
import { mkdtemp } from 'node:fs/promises'
import { tmpdir } from 'node:os'

it('starts bundled Prime through a private SSH Unix socket and stops it with its session', async context => {
  const available = await detectSshLinuxFixture()
  if (!available.available) { context.skip(available.reason); return }
  const fixture = await createSshLinuxFixture()
  const local = await mkdtemp(join(tmpdir(), 'prime-ssh-runtime-'))
  let runtime: RemoteRuntime | undefined
  const manager = new RemoteManager({ path: join(local, 'profiles.json'), emit: () => {}, onReady: async (profile, connection) => {
    runtime = await RemoteRuntime.start(profile, connection, { runtimeDir: resolve('runtime'), stateDirectory: fixture.linuxDir + '/desktop-runtime', environment: { PRIME_GLOBAL_CONFIG: fixture.linuxDir + '/config', PRIME_GLOBAL_DATA: fixture.linuxDir + '/data', PRIME_CACHE_DIR: fixture.linuxDir + '/cache' }, progress: () => {} })
  }, onDisconnect: async () => { await runtime?.close() } })
  try {
    await manager.load()
    const saved = await manager.save({ name: 'Runtime fixture', host: fixture.host, port: fixture.port, user: fixture.user, root: fixture.root, auth: 'key', keyPath: fixture.keyPath, runtime: 'bundled' })
    const id = saved.profiles[0].id
    const challenged = await manager.connect(id)
    await manager.trust(id, challenged.profiles[0].challenge!.fingerprint)
    await manager.connect(id)
    expect(manager.list().profiles[0].status).toBe('connected')
    await expect(runtime!.transport.request('GET', '/v1/health')).resolves.toBeDefined()
    expect((await runtime!.transport.request('GET', '/v1/capabilities')).protocol_version).toBe(1)
    const socket = runtime!.socketPath
    await manager.disconnect(id)
    expect(await fixture.execute(`test ! -e '${socket}' && printf cleaned`)).toBe('cleaned')
  } finally { await manager.shutdown(); await fixture.close() }
}, 120000)

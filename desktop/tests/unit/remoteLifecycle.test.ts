import { describe, expect, it } from 'vitest'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createSshLinuxFixture, detectSshLinuxFixture } from '../fixtures/ssh-linux'
import { RemoteManager } from '../../src/main/remote/manager'
const support = await detectSshLinuxFixture()
describe.skipIf(!support.available)('remote setup cancellation', () => {
  it.each(['disconnect', 'lost'] as const)(
    'waits for in-flight preparation before final cleanup after %s',
    async (mode) => {
      const fixture = await createSshLinuxFixture()
      const directory = await mkdtemp(join(tmpdir(), 'prime-remote-race-'))
      let finish!: () => void
      const gate = new Promise<void>((resolve) => {
        finish = resolve
      })
      let started!: () => void
      const entered = new Promise<void>((resolve) => {
        started = resolve
      })
      let resource = false
      let lost!: () => void
      const connectionLost = new Promise<void>((resolve) => {
        lost = resolve
      })
      const manager = new RemoteManager({
        path: join(directory, 'profiles.json'),
        emit() {},
        onReady: async () => {
          started()
          await gate
          resource = true
        },
        onLost: () => lost(),
        onDisconnect: async () => {
          resource = false
        }
      })
      try {
        await manager.load()
        const saved = await manager.save({
          name: 'Linux',
          host: fixture.host,
          port: fixture.port,
          user: fixture.user,
          root: fixture.root,
          auth: 'key',
          keyPath: fixture.keyPath,
          runtime: 'bundled'
        })
        const id = saved.profiles[0].id
        await manager.connect(id)
        await manager.trust(id, manager.list().profiles[0].challenge!.fingerprint)
        const connecting = manager.connect(id)
        await entered
        const disconnecting =
          mode === 'disconnect'
            ? manager.disconnect(id)
            : (async () => {
                manager.connection(id).client.destroy()
                await connectionLost
              })()
        if (mode === 'lost') await disconnecting
        finish()
        await Promise.all([connecting, disconnecting])
        expect(resource).toBe(false)
        expect(manager.list().profiles[0].status).toBe(
          mode === 'disconnect' ? 'disconnected' : 'error'
        )
      } finally {
        finish()
        await manager.shutdown()
        await fixture.close()
        await rm(directory, { recursive: true, force: true })
      }
    },
    60000
  )
})

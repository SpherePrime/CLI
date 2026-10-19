import { describe, expect, it, vi } from 'vitest'
import { createHash } from 'node:crypto'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createSshLinuxFixture, detectSshLinuxFixture } from '../fixtures/ssh-linux'
import { RemoteManager } from '../../src/main/remote/manager'
const support = await detectSshLinuxFixture()
describe.skipIf(!support.available)('remote Linux SFTP and shell', () => {
  it('runs real file operations with boundaries, revisions and reconnect', async () => {
    const fixture = await createSshLinuxFixture()
    const directory = await mkdtemp(join(tmpdir(), 'prime-remote-real-'))
    const manager = new RemoteManager({ path: join(directory, 'profiles.json'), emit() {} })
    try {
      await fixture.execute(
        'printf original > file.txt; mkdir folder; ln -s /etc outside; git init -q'
      )
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
      const challenge = manager.list().profiles[0].challenge!
      expect(challenge.fingerprint).toMatch(/^SHA256:/)
      await manager.trust(id, challenge.fingerprint)
      await manager.connect(id)
      expect(manager.list().profiles[0].status).toBe('connected')
      const connection = manager.connection(id)
      expect((await connection.list()).map((entry) => entry.name)).toContain('file.txt')
      const original = await connection.readFile('file.txt')
      expect(original.content).toBe('original')
      await fixture.execute('printf external > file.txt')
      await expect(connection.saveFile('file.txt', 'edited', original.revision!)).rejects.toThrow(
        'remote.revisionConflict'
      )
      const external = await connection.readFile('file.txt')
      expect((await connection.saveFile('file.txt', 'edited', external.revision!)).content).toBe(
        'edited'
      )
      const rename = connection.sftp.ext_openssh_rename.bind(connection.sftp)
      const renameHook = vi
        .spyOn(connection.sftp, 'ext_openssh_rename')
        .mockImplementation((from, to, callback) =>
          rename(from, to, (error) => {
            void fixture
              .execute('printf external-after-rename > file.txt')
              .then(() => callback(error))
          })
        )
      const beforeRename = await connection.readFile('file.txt')
      const submitted = await connection.saveFile('file.txt', 'submitted', beforeRename.revision!)
      renameHook.mockRestore()
      expect(submitted.revision).toBe(createHash('sha256').update('submitted').digest('hex'))
      await expect(
        connection.saveFile('file.txt', 'overwrite', submitted.revision!)
      ).rejects.toThrow('remote.revisionConflict')
      const latest = await connection.readFile('file.txt')
      const writes = await Promise.allSettled([
        connection.saveFile('file.txt', 'first', latest.revision!),
        connection.saveFile('file.txt', 'second', latest.revision!)
      ])
      expect(writes.filter((write) => write.status === 'fulfilled')).toHaveLength(1)
      const final = await connection.readFile('file.txt')
      await connection.saveFile('file.txt', 'edited', final.revision!)
      await expect(connection.readFile('outside/passwd')).rejects.toThrow('remote.pathOutside')
      await expect(connection.createEntry('outside/new-file', false)).rejects.toThrow(
        'remote.pathOutside'
      )
      await expect(connection.readFile('../etc/passwd')).rejects.toThrow('remote.pathOutside')
      await expect(connection.removeEntry('.')).rejects.toThrow('remote.pathOutside')
      await expect(connection.removeEntry('')).rejects.toThrow('remote.pathOutside')
      await connection.createEntry('new.txt', false)
      await connection.removeEntry('new.txt')
      await fixture.execute('ln -s file.txt linked.txt')
      await connection.removeEntry('linked.txt')
      expect((await connection.readFile('file.txt')).content).toBe('edited')
      expect((await connection.gitChanges()).map((entry) => entry.path)).toContain('file.txt')
      const shell = await connection.shell(80, 24)
      const output = await new Promise<string>((resolve, reject) => {
        let text = ''
        const timeout = setTimeout(() => {
          shell.close()
          reject(new Error('shell timeout'))
        }, 5000)
        shell.on('data', (chunk: Buffer) => {
          text += chunk.toString()
          if (text.includes('REMOTE_ROUNDTRIP')) {
            clearTimeout(timeout)
            resolve(text)
          }
        })
        shell.write('printf \'REMOTE_\'"ROUNDTRIP\\n"\n')
      })
      expect(output).toContain('REMOTE_ROUNDTRIP')
      shell.close()
      await manager.disconnect(id)
      expect(manager.list().profiles[0].status).toBe('disconnected')
      await manager.connect(id)
      expect(manager.list().profiles[0].status).toBe('connected')
      await manager.fail(id, 'secret private error')
      expect(manager.list().profiles[0].status).toBe('error')
      expect(manager.list().profiles[0].error).toBe('remote.runtimeFailed')
      expect(() => manager.connection(id)).toThrow('remote.notConnected')
    } finally {
      await manager.shutdown()
      await fixture.close()
      await rm(directory, { recursive: true, force: true })
    }
  }, 60000)
})

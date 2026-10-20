import { afterEach, describe, expect, it, vi } from 'vitest'
import { mkdtemp, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { RemoteConnection } from '../../src/main/remote/connection'
import { migrateRemoteHistory } from '../../src/main/remote/historyMigration'

const directories: string[] = []
afterEach(async () => {
  await Promise.all(
    directories.splice(0).map((directory) => rm(directory, { recursive: true, force: true }))
  )
})

async function localRoot() {
  const directory = await mkdtemp(join(tmpdir(), 'prime-history-'))
  directories.push(directory)
  return directory
}

function connection(history = true) {
  const snapshot = Buffer.concat([Buffer.from('SQLite format 3\0'), Buffer.alloc(4080)])
  const remote = {
    root: '/project',
    resolve: vi.fn(async () => {
      if (!history) throw Object.assign(new Error('missing'), { code: 2 })
      return '/project/.prime/prime.db'
    }),
    exec: vi.fn(async (_command: string) => ({
      code: 0,
      stdout: '',
      stderr: ''
    })),
    sftp: {
      mkdir: vi.fn((_path, _options, done) => done()),
      unlink: vi.fn((_path, done) => done()),
      rmdir: vi.fn((_path, done) => done()),
      fastGet: vi.fn(
        (_path, destination, done) => void writeFile(destination, snapshot).then(() => done(), done)
      ),
      stat: vi.fn((_path, done) => done(null, { isFile: () => true, size: snapshot.length }))
    }
  }
  return { remote: remote as unknown as RemoteConnection, mocks: remote, snapshot }
}

describe('remote chat history migration', () => {
  it('uses the registered custom history directory without importing provider files', async () => {
    const root = await localRoot()
    const { remote, mocks } = connection()
    mocks.exec.mockResolvedValueOnce({
      code: 0,
      stdout: JSON.stringify({ projects: [{ path: '/project', data_dir: '/custom/history' }] }),
      stderr: ''
    })
    expect(await migrateRemoteHistory(remote, root)).toEqual({ migrated: true, reason: 'migrated' })
    expect(mocks.resolve).not.toHaveBeenCalled()
    expect(mocks.sftp.stat.mock.calls[0][0]).toBe('/custom/history/prime.db')
    expect(mocks.exec.mock.calls[1][0]).toContain('/custom/history/prime.db')
  })

  it('installs only a database snapshot and preserves local configuration', async () => {
    const root = await localRoot()
    await mkdir(join(root, '.prime'))
    await writeFile(join(root, '.prime', 'providers.json'), 'local-secret')
    const { remote, mocks, snapshot } = connection()
    expect(await migrateRemoteHistory(remote, root)).toEqual({ migrated: true, reason: 'migrated' })
    expect(await readFile(join(root, '.prime', 'prime.db'))).toEqual(snapshot)
    expect(await readFile(join(root, '.prime', 'providers.json'), 'utf8')).toBe('local-secret')
    expect((await readdir(join(root, '.prime'))).sort()).toEqual(['prime.db', 'providers.json'])
    expect(mocks.exec.mock.calls.some(([command]) => command.includes('backup'))).toBe(true)
    expect(mocks.sftp.rmdir).toHaveBeenCalledOnce()
  })

  it('does not contact the server or overwrite existing local history', async () => {
    const root = await localRoot()
    await mkdir(join(root, '.prime'))
    await writeFile(join(root, '.prime', 'prime.db'), 'existing-history')
    const { remote, mocks } = connection()
    expect(await migrateRemoteHistory(remote, root)).toEqual({
      migrated: false,
      reason: 'local-history-exists'
    })
    expect(mocks.resolve).not.toHaveBeenCalled()
    expect(mocks.exec).not.toHaveBeenCalled()
    expect(await readFile(join(root, '.prime', 'prime.db'), 'utf8')).toBe('existing-history')
  })

  it('opens new projects without requiring a remote SQLite runtime', async () => {
    const root = await localRoot()
    const { remote, mocks } = connection(false)
    expect(await migrateRemoteHistory(remote, root)).toEqual({
      migrated: false,
      reason: 'no-history'
    })
    expect(mocks.sftp.fastGet).not.toHaveBeenCalled()
    expect(mocks.exec.mock.calls.every(([command]) => !command.includes('backup'))).toBe(true)
  })

  it('keeps local history absent and cleans snapshots when backup fails', async () => {
    const root = await localRoot()
    const { remote, mocks } = connection()
    mocks.exec
      .mockResolvedValueOnce({ code: 0, stdout: '', stderr: '' })
      .mockResolvedValue({ code: 71, stdout: '', stderr: '' })
    await expect(migrateRemoteHistory(remote, root)).rejects.toThrow(
      'remote.historySnapshotUnavailable'
    )
    expect(mocks.sftp.fastGet).not.toHaveBeenCalled()
    expect(mocks.sftp.rmdir).toHaveBeenCalledOnce()
    await expect(readFile(join(root, '.prime', 'prime.db'))).rejects.toMatchObject({
      code: 'ENOENT'
    })
  })
})

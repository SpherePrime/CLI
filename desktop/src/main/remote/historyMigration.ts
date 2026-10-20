import { randomUUID } from 'node:crypto'
import { link, lstat, mkdir, open, rm, writeFile } from 'node:fs/promises'
import { join, posix } from 'node:path'
import type { RemoteConnection } from './connection'
import { historySnapshotCommand } from './historyMigrationSnapshot'

export type HistoryMigrationResult = {
  migrated: boolean
  reason: 'migrated' | 'no-history' | 'local-history-exists'
}

function missing(error: unknown) {
  return [2, 'ENOENT'].includes((error as NodeJS.ErrnoException)?.code ?? '')
}

async function exists(path: string) {
  try {
    await lstat(path)
    return true
  } catch (error) {
    if (!missing(error)) throw error
    return false
  }
}

async function sourceDatabase(connection: RemoteConnection): Promise<string | undefined> {
  const registry = await connection.exec(
    [
      'if [ -n "$PRIME_GLOBAL_DATA" ]; then directory=$PRIME_GLOBAL_DATA;',
      'elif [ -n "$XDG_DATA_HOME" ]; then directory=$XDG_DATA_HOME/prime;',
      'else directory=$HOME/.local/share/prime; fi',
      'if [ -f "$directory/projects.json" ]; then cat "$directory/projects.json"; fi'
    ].join('\n'),
    { maxBytes: 2 * 1024 * 1024 }
  )
  if (registry.code) throw new Error('remote.historyReadFailed')
  if (registry.stdout.trim()) {
    let projects: { path?: unknown; data_dir?: unknown }[]
    try {
      projects = JSON.parse(registry.stdout).projects
      if (!Array.isArray(projects)) throw new Error()
    } catch {
      throw new Error('remote.historyReadFailed')
    }
    const project = projects.find((entry) => entry.path === connection.root)
    if (
      typeof project?.data_dir === 'string' &&
      posix.isAbsolute(project.data_dir) &&
      !project.data_dir.includes('\0')
    ) {
      const source = posix.join(project.data_dir, 'prime.db')
      try {
        const info = await new Promise<import('ssh2').Stats>((resolve, reject) =>
          connection.sftp.stat(source, (error, result) => (error ? reject(error) : resolve(result)))
        )
        if (!info.isFile()) throw new Error('remote.historyReadFailed')
        return source
      } catch (error) {
        if (!missing(error)) throw error
      }
    }
  }
  try {
    return await connection.resolve('.prime/prime.db')
  } catch (error) {
    if (!missing(error)) throw error
    return undefined
  }
}

export async function migrateRemoteHistory(
  connection: RemoteConnection,
  localRoot: string
): Promise<HistoryMigrationResult> {
  const directory = join(localRoot, '.prime')
  const destination = join(directory, 'prime.db')
  for (const suffix of ['', '-wal', '-shm', '-journal']) {
    if (await exists(destination + suffix))
      return { migrated: false, reason: 'local-history-exists' }
  }
  const source = await sourceDatabase(connection)
  if (!source) return { migrated: false, reason: 'no-history' }
  const identity = randomUUID()
  const remoteDirectory = posix.join(posix.dirname(source), `.history-${identity}`)
  const remoteSnapshot = posix.join(remoteDirectory, 'prime.db')
  const temporary = join(directory, `.history-${identity}.tmp`)
  await new Promise<void>((resolve, reject) =>
    connection.sftp.mkdir(remoteDirectory, { mode: 0o700 }, (error) =>
      error ? reject(error) : resolve()
    )
  )
  try {
    const backup = await connection.exec(historySnapshotCommand(source, remoteDirectory), {
      timeout: 60000,
      maxBytes: 4096
    })
    if (backup.code === 71) throw new Error('remote.historySnapshotUnavailable')
    if (backup.code) throw new Error('remote.historySnapshotFailed')
    await mkdir(directory, { recursive: true, mode: 0o700 })
    const info = await lstat(directory)
    if (!info.isDirectory() || info.isSymbolicLink())
      throw new Error('remote.historyInvalidDestination')
    await writeFile(temporary, '', { flag: 'wx', mode: 0o600 })
    await new Promise<void>((resolve, reject) =>
      connection.sftp.fastGet(remoteSnapshot, temporary, (error) =>
        error ? reject(error) : resolve()
      )
    )
    const file = await open(temporary, 'r+')
    try {
      const header = Buffer.alloc(16)
      const result = await file.read(header, 0, 16, 0)
      if (
        result.bytesRead !== 16 ||
        !header.equals(Buffer.from('SQLite format 3\0')) ||
        (await file.stat()).size < 100
      )
        throw new Error('remote.historySnapshotFailed')
      await file.sync()
    } finally {
      await file.close()
    }
    try {
      await link(temporary, destination)
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === 'EEXIST')
        return { migrated: false, reason: 'local-history-exists' }
      throw error
    }
    return { migrated: true, reason: 'migrated' }
  } finally {
    await rm(temporary, { force: true }).catch(() => {})
    for (const suffix of ['', '-wal', '-shm', '-journal']) {
      await new Promise<void>((resolve) =>
        connection.sftp.unlink(remoteSnapshot + suffix, () => resolve())
      )
    }
    await new Promise<void>((resolve) => connection.sftp.rmdir(remoteDirectory, () => resolve()))
  }
}

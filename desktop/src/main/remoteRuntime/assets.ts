import { createHash, randomUUID } from 'node:crypto'
import { createReadStream } from 'node:fs'
import { readFile, stat } from 'node:fs/promises'
import { join, posix } from 'node:path'
import type { RemoteConnection } from '../remote/connection'
import type { SFTPWrapper } from 'ssh2'

type Manifest = { version: string; commit: string; platform: string; arch: string; protocolVersion: number; sha256: string }
const call = <T>(operation: (callback: (error?: Error | null, result?: T) => void) => void): Promise<T> => new Promise((resolve, reject) => operation((error, result) => error ? reject(error) : resolve(result!)))

export async function remoteDirectory(sftp: SFTPWrapper, path: string): Promise<void> {
  if (!posix.isAbsolute(path) || path.includes('\0')) throw new Error('remote.runtimeFailed')
  let current = ''
  for (const part of path.split('/').filter(Boolean)) {
    current += '/' + part
    try { await call(callback => sftp.mkdir(current, { mode: 0o700 }, callback)) }
    catch (error) {
      const info = await call<import('ssh2').Stats>(callback => sftp.stat(current, callback))
      if (!info.isDirectory()) throw error
    }
  }
}

async function cachedManifest(sftp: SFTPWrapper, path: string): Promise<Manifest | undefined> {
  try {
    const bytes = await new Promise<Buffer>((resolve, reject) => {
      const chunks: Buffer[] = []
      let length = 0
      const stream = sftp.createReadStream(path)
      stream.on('data', (chunk: Buffer) => { length += chunk.length; if (length > 65536) stream.destroy(new Error('remote.runtimeFailed')); else chunks.push(chunk) })
      stream.once('error', reject).once('end', () => resolve(Buffer.concat(chunks)))
    })
    return JSON.parse(bytes.toString('utf8'))
  } catch { return undefined }
}

export async function installRemoteRuntime(connection: RemoteConnection, options: { runtimeDir: string; base: string; arch: 'x64' | 'arm64'; progress(message: string, percent?: number): void }): Promise<string> {
  const directory = join(options.runtimeDir, 'remotes', `linux-${options.arch}`)
  const binary = join(directory, 'prime')
  const manifest: Manifest = JSON.parse(await readFile(join(directory, 'manifest.json'), 'utf8'))
  const local: Manifest = JSON.parse(await readFile(join(options.runtimeDir, 'manifest.json'), 'utf8'))
  const hash = createHash('sha256')
  for await (const chunk of createReadStream(binary)) hash.update(chunk)
  if (manifest.platform !== 'linux' || manifest.arch !== options.arch || manifest.protocolVersion !== 1 || manifest.commit !== local.commit || manifest.version !== local.version || manifest.sha256 !== hash.digest('hex')) throw new Error('remote.runtimeUnavailable')
  const targetDirectory = posix.join(options.base, 'runtime', `linux-${options.arch}`)
  const target = posix.join(targetDirectory, 'prime')
  const manifestPath = posix.join(targetDirectory, 'manifest.json')
  const sftp = connection.sftp
  await remoteDirectory(sftp, targetDirectory)
  const cached = await cachedManifest(sftp, manifestPath)
  const size = (await stat(binary)).size
  if (cached?.sha256 === manifest.sha256) {
    try {
      const existing = await call<import('ssh2').Stats>(callback => sftp.stat(target, callback))
      if (existing.isFile() && existing.size === size && (existing.mode & 0o111)) return target
    } catch {}
  }
  const temporary = target + '.' + randomUUID() + '.tmp'
  const temporaryManifest = manifestPath + '.' + randomUUID() + '.tmp'
  try {
    await call(callback => sftp.fastPut(binary, temporary, { mode: 0o700, step: transferred => options.progress('Загрузка Prime на сервер', transferred / size * 100) }, callback))
    await call(callback => sftp.chmod(temporary, 0o700, callback))
    await call(callback => sftp.ext_openssh_rename(temporary, target, callback))
    await call(callback => sftp.writeFile(temporaryManifest, JSON.stringify(manifest), { mode: 0o600, flag: 'wx' }, callback))
    await call(callback => sftp.ext_openssh_rename(temporaryManifest, manifestPath, callback))
    return target
  } finally {
    await Promise.allSettled([call(callback => sftp.unlink(temporary, callback)), call(callback => sftp.unlink(temporaryManifest, callback))])
  }
}

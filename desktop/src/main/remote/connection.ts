import { createHash, randomUUID } from 'node:crypto'
import { posix } from 'node:path'
import type { Client, SFTPWrapper, Stats } from 'ssh2'
import ignore from 'ignore'
import { createTwoFilesPatch } from 'diff'
import type { RemoteProfile } from '../../shared/remote'
import { remotePath, shellQuote } from './security'
const limit = 2 * 1024 * 1024
export class RemoteConnection {
  root = ''
  private writes: Promise<unknown> = Promise.resolve()
  constructor(
    readonly client: Client,
    readonly sftp: SFTPWrapper,
    readonly profile: RemoteProfile
  ) {}
  private call<T>(
    operation: (done: (error: Error | undefined | null, result: T) => void) => void
  ): Promise<T> {
    return new Promise((resolve, reject) =>
      operation((error, result) => (error ? reject(error) : resolve(result)))
    )
  }
  async initialize() {
    this.root = await this.call<string>((done) => this.sftp.realpath(this.profile.root, done))
    if (!(await this.call<Stats>((done) => this.sftp.stat(this.root, done))).isDirectory())
      throw new Error('remote.notDirectory')
    return this
  }
  private inside(path: string) {
    if (path !== this.root && !path.startsWith(this.root.replace(/\/$/, '') + '/'))
      throw new Error('remote.pathOutside')
    return path
  }
  async resolve(path: string, missing = false): Promise<string> {
    const absolute = remotePath(this.root, path)
    try {
      return this.inside(await this.call<string>((done) => this.sftp.realpath(absolute, done)))
    } catch (error: any) {
      if (!missing || ![2, 'ENOENT'].includes(error.code)) throw error
      const parent = this.inside(
        await this.call<string>((done) => this.sftp.realpath(posix.dirname(absolute), done))
      )
      return posix.join(parent, posix.basename(absolute))
    }
  }
  async stat(path: string) {
    const absolute = await this.resolve(path)
    return this.call<Stats>((done) => this.sftp.stat(absolute, done))
  }
  async realpath(path: string) {
    return this.resolve(path)
  }
  private async bytes(absolute: string): Promise<Buffer> {
    const info = await this.call<Stats>((done) => this.sftp.stat(absolute, done))
    if (!info.isFile()) throw new Error('remote.notFile')
    if (info.size > limit) throw new Error('remote.fileTooLarge')
    return new Promise((resolve, reject) => {
      const chunks: Buffer[] = []
      let size = 0
      const stream = this.sftp.createReadStream(absolute)
      stream.on('data', (chunk: Buffer) => {
        size += chunk.length
        if (size > limit) stream.destroy(new Error('remote.fileTooLarge'))
        else chunks.push(chunk)
      })
      stream.on('error', reject)
      stream.on('end', () => resolve(Buffer.concat(chunks)))
    })
  }
  async list(path = '') {
    const absolute = await this.resolve(path)
    const entries = await this.call<import('ssh2').FileEntryWithStats[]>((done) =>
      this.sftp.readdir(absolute, done)
    )
    const rules = ignore()
    for (const name of ['.gitignore', '.primeignore']) {
      try {
        rules.add((await this.bytes(await this.resolve(name))).toString('utf8'))
      } catch (error: any) {
        if (![2, 'ENOENT'].includes(error.code)) throw error
      }
    }
    return entries
      .filter(
        (entry) =>
          !['.', '..', '.git', 'node_modules', '.superpowers', '.env', '.env.local'].includes(
            entry.filename
          ) &&
          !rules.ignores(posix.join(path, entry.filename) + (entry.attrs.isDirectory() ? '/' : ''))
      )
      .map((entry) => ({
        name: entry.filename,
        path: posix.join(path, entry.filename),
        directory: entry.attrs.isDirectory()
      }))
      .sort((a, b) => Number(b.directory) - Number(a.directory) || a.name.localeCompare(b.name))
  }
  async readFile(path: string) {
    const bytes = await this.bytes(await this.resolve(path))
    const mime: Record<string, string> = {
      '.png': 'image/png',
      '.jpg': 'image/jpeg',
      '.jpeg': 'image/jpeg',
      '.gif': 'image/gif',
      '.webp': 'image/webp'
    }
    const type = mime[posix.extname(path).toLowerCase()]
    if (type)
      return {
        path,
        name: posix.basename(path),
        image: `data:${type};base64,${bytes.toString('base64')}`
      }
    if (bytes.includes(0)) throw new Error('remote.binaryFile')
    return {
      path,
      name: posix.basename(path),
      content: new TextDecoder('utf-8', { fatal: true }).decode(bytes),
      revision: createHash('sha256').update(bytes).digest('hex')
    }
  }
  saveFile(path: string, content: string, revision: string) {
    const operation = this.writes.then(() => this.writeRevision(path, content, revision))
    this.writes = operation.catch(() => {})
    return operation
  }
  createFile(path: string, content: string) {
    const operation = this.writes.then(async () => {
      if (Buffer.byteLength(content) > limit) throw new Error('remote.fileTooLarge')
      const absolute = await this.resolve(path, true)
      await this.call<void>((done) =>
        this.sftp.writeFile(absolute, content, { flag: 'wx', mode: 0o644 }, done)
      )
      return { path, revision: createHash('sha256').update(content).digest('hex') }
    })
    this.writes = operation.catch(() => {})
    return operation
  }
  private async writeRevision(path: string, content: string, revision: string) {
    if (Buffer.byteLength(content) > limit) throw new Error('remote.fileTooLarge')
    const absolute = await this.resolve(path)
    const current = await this.readFile(path)
    if (!revision || current.revision !== revision) throw new Error('remote.revisionConflict')
    const mode = (await this.stat(path)).mode & 0o777
    const temporary = posix.join(posix.dirname(absolute), `.prime-${randomUUID()}.tmp`)
    try {
      await this.call<void>((done) =>
        this.sftp.writeFile(temporary, content, { flag: 'wx', mode }, done)
      )
      if ((await this.readFile(path)).revision !== revision)
        throw new Error('remote.revisionConflict')
      if ((await this.resolve(path)) !== absolute) throw new Error('remote.pathOutside')
      await this.call<void>((done) => this.sftp.ext_openssh_rename(temporary, absolute, done))
      return {
        path,
        name: posix.basename(path),
        content,
        revision: createHash('sha256').update(content).digest('hex')
      }
    } finally {
      await this.call<void>((done) => this.sftp.unlink(temporary, done)).catch(() => {})
    }
  }
  async createEntry(path: string, directory: boolean) {
    const absolute = await this.resolve(path, true)
    if (directory) await this.call<void>((done) => this.sftp.mkdir(absolute, { mode: 0o755 }, done))
    else
      await this.call<void>((done) =>
        this.sftp.writeFile(absolute, '', { flag: 'wx', mode: 0o644 }, done)
      )
  }
  async removeEntry(path: string) {
    if (!path) throw new Error('remote.pathOutside')
    const lexical = remotePath(this.root, path)
    if (lexical === this.root) throw new Error('remote.pathOutside')
    const parent = this.inside(
      await this.call<string>((done) => this.sftp.realpath(posix.dirname(lexical), done))
    )
    const absolute = posix.join(parent, posix.basename(lexical))
    const info = await this.call<Stats>((done) => this.sftp.lstat(absolute, done))
    if (info.isDirectory()) await this.call<void>((done) => this.sftp.rmdir(absolute, done))
    else await this.call<void>((done) => this.sftp.unlink(absolute, done))
  }
  exec(
    command: string,
    options: { timeout?: number; maxBytes?: number } = {}
  ): Promise<{ stdout: string; stderr: string; code: number }> {
    return new Promise((resolve, reject) => {
      let channel: import('ssh2').ClientChannel | undefined
      let settled = false
      let cleanup = () => {}
      const timer = setTimeout(() => {
        if (settled) return
        settled = true
        cleanup()
        channel?.close()
        reject(new Error('remote.commandTimeout'))
      }, options.timeout ?? 30000)
      this.client.exec(command, (error, stream) => {
        if (settled) {
          stream?.close()
          return
        }
        if (error) {
          settled = true
          clearTimeout(timer)
          reject(error)
          return
        }
        channel = stream
        const stdout: Buffer[] = []
        const stderr: Buffer[] = []
        let size = 0
        const collect = (target: Buffer[]) => (chunk: Buffer) => {
          if (settled) return
          size += chunk.length
          if (size > (options.maxBytes ?? 4 * 1024 * 1024)) {
            settled = true
            clearTimeout(timer)
            cleanup()
            stream.close()
            reject(new Error('remote.outputTooLarge'))
          } else target.push(Buffer.from(chunk))
        }
        const onData = collect(stdout)
        const onErrorData = collect(stderr)
        stream.on('data', onData)
        stream.stderr.on('data', onErrorData)
        cleanup = () => {
          stream.removeListener('data', onData)
          stream.stderr.removeListener('data', onErrorData)
        }
        stream.on('error', (failure: Error) => {
          if (settled) return
          settled = true
          clearTimeout(timer)
          cleanup()
          stream.close()
          reject(failure)
        })
        stream.on('close', (code: number) => {
          stream.removeListener('data', onData)
          stream.stderr.removeListener('data', onErrorData)
          if (settled) return
          settled = true
          clearTimeout(timer)
          resolve({
            stdout: Buffer.concat(stdout).toString('utf8'),
            stderr: Buffer.concat(stderr).toString('utf8'),
            code: code ?? -1
          })
        })
      })
    })
  }
  shell(cols = 80, rows = 24) {
    return this.call<import('ssh2').ClientChannel>((done) =>
      this.client.shell({ cols, rows, term: 'xterm-256color' }, done)
    )
  }
  async gitChanges() {
    const prefixResult = await this.exec(
      `cd ${shellQuote(this.root)} && git rev-parse --show-prefix`
    )
    if (prefixResult.code) throw new Error('remote.gitFailed')
    const prefix = prefixResult.stdout.trim()
    const result = await this.exec(
      `cd ${shellQuote(this.root)} && git status --porcelain=v1 -z --untracked-files=all -- .`
    )
    if (result.code) throw new Error('remote.gitFailed')
    const entries = result.stdout.split('\0')
    const changes: { path: string; status: string; originalPath?: string }[] = []
    for (let i = 0; i < entries.length; i++) {
      const entry = entries[i]
      if (!entry) continue
      const status = entry.slice(0, 2)
      const path = entry.slice(3)
      const original = /[RC]/.test(status) ? entries[++i] : undefined
      if (!path.startsWith(prefix)) continue
      changes.push({
        path: path.slice(prefix.length),
        status,
        originalPath: original?.startsWith(prefix) ? original.slice(prefix.length) : undefined
      })
    }
    return changes
  }
  async gitDiff(path: string) {
    remotePath(this.root, path)
    const change = (await this.gitChanges()).find((item) => item.path === path)
    let after = ''
    try {
      after = (await this.readFile(path)).content ?? ''
    } catch (error: any) {
      if (![2, 'ENOENT'].includes(error.code)) throw error
    }
    let before = ''
    if (change?.status !== '??' && !change?.status.includes('A')) {
      const result = await this.exec(
        `cd ${shellQuote(this.root)} && git show ${shellQuote('HEAD:./' + (change?.originalPath || path))}`
      )
      if (result.code) throw new Error('remote.gitFailed')
      before = result.stdout
    }
    return {
      path,
      source: 'git',
      before,
      after,
      status: change?.status,
      originalPath: change?.originalPath,
      patch: createTwoFilesPatch(change?.originalPath || path, path, before, after)
    }
  }
  close() {
    this.sftp.end()
    this.client.end()
  }
}

import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { readFile, stat } from 'node:fs/promises'
import { resolve, relative } from 'node:path'
import { createTwoFilesPatch } from 'diff'
import { resolveProjectPath } from './paths'
const run = promisify(execFile)
export type GitChange = { path: string; status: string; originalPath?: string }
export async function gitChanges(root: string): Promise<GitChange[]> {
  const options = { cwd: root, windowsHide: true, maxBuffer: 8 * 1024 * 1024 }
  const prefix = (await run('git', ['rev-parse', '--show-prefix'], options)).stdout.trim()
  const output = (
    await run(
      'git',
      ['status', '--porcelain=v1', '-z', '--untracked-files=all', '--', '.'],
      options
    )
  ).stdout.split('\0')
  const changes: GitChange[] = []
  for (let index = 0; index < output.length; index++) {
    const entry = output[index]
    if (!entry) continue
    const status = entry.slice(0, 2)
    const repositoryPath = entry.slice(3)
    const original = /[RC]/.test(status) ? output[++index] : undefined
    if (prefix && !repositoryPath.startsWith(prefix)) continue
    changes.push({
      path: repositoryPath.slice(prefix.length),
      status,
      originalPath: original?.startsWith(prefix) ? original.slice(prefix.length) : undefined
    })
  }
  return changes
}
export async function gitFileDiff(root: string, path: string): Promise<any> {
  const absolute = await resolveProjectPath(root, path, true)
  let after = ''
  try {
    const info = await stat(absolute)
    if (!info.isFile() || info.size > 2 * 1024 * 1024)
      throw new Error('Предпросмотр недоступен для каталога или файла больше 2 MiB')
    const bytes = await readFile(absolute)
    if (bytes.includes(0)) throw new Error('Бинарный файл: откройте его во внешнем редакторе')
    after = bytes.toString('utf8')
  } catch (error: any) {
    if (error.code !== 'ENOENT') throw error
  }
  const options = { cwd: root, windowsHide: true, maxBuffer: 4 * 1024 * 1024 }
  const changes = await gitChanges(root)
  const change = changes.find(
    (item) => item.path.replaceAll('\\', '/') === path.replaceAll('\\', '/')
  )
  let before = ''
  if (change?.status !== '??' && !change?.status.includes('A')) {
    const original = change?.originalPath || path
    const objectPath = relative(root, resolve(root, original)).replaceAll('\\', '/')
    try {
      before = (await run('git', ['show', `HEAD:./${objectPath}`], options)).stdout
    } catch (error: any) {
      if (
        !/does not exist|exists on disk|invalid object name|bad revision/i.test(error.stderr || '')
      )
        throw new Error('Не удалось прочитать Git HEAD')
    }
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

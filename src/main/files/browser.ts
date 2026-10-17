import { readFile, readdir, stat } from 'node:fs/promises'
import { basename, extname, join } from 'node:path'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { createTwoFilesPatch } from 'diff'
import { resolveProjectPath } from './paths'
const run = promisify(execFile)
export async function listFiles(root: string, path = ''): Promise<any[]> {
  const absolute = await resolveProjectPath(root, path)
  const entries = await readdir(absolute, { withFileTypes: true })
  return entries
    .filter(
      (entry) =>
        !['.git', 'node_modules', '.superpowers', '.env', '.env.local'].includes(entry.name)
    )
    .map((entry) => ({
      name: entry.name,
      path: join(path, entry.name),
      directory: entry.isDirectory()
    }))
    .sort((a, b) => Number(b.directory) - Number(a.directory) || a.name.localeCompare(b.name))
}
export async function readProjectFile(root: string, path: string): Promise<any> {
  const absolute = await resolveProjectPath(root, path)
  const info = await stat(absolute)
  if (!info.isFile()) throw new Error('Выберите файл')
  if (info.size > 2 * 1024 * 1024)
    throw new Error('Файл больше 2 MiB. Откройте его во внешнем редакторе.')
  const buffer = await readFile(absolute)
  const imageTypes: Record<string, string> = {
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.webp': 'image/webp',
    '.gif': 'image/gif'
  }
  const mime = imageTypes[extname(path).toLowerCase()]
  if (mime)
    return { path, name: basename(path), image: `data:${mime};base64,${buffer.toString('base64')}` }
  if (buffer.includes(0)) throw new Error('Бинарный файл — откройте его во внешнем приложении')
  return { path, name: basename(path), content: buffer.toString('utf8') }
}
export async function fileDiff(
  root: string,
  path: string,
  source: 'session' | 'git',
  history: any[]
): Promise<any> {
  let after = ''
  try {
    after = (await readProjectFile(root, path)).content || ''
  } catch (error: any) {
    if (error.code !== 'ENOENT') throw error
  }
  let before = ''
  if (source === 'git') {
    try {
      before = (
        await run('git', ['show', `HEAD:${path.replaceAll('\\', '/')}`], {
          cwd: root,
          windowsHide: true,
          maxBuffer: 4 * 1024 * 1024
        })
      ).stdout
    } catch (error: any) {
      if (!error.stderr?.includes('exists on disk') && !error.stderr?.includes('does not exist'))
        throw new Error('Git-сравнение недоступно: проверьте репозиторий и установленный Git')
    }
  } else {
    const files = history
      .filter(
        (file) =>
          file.path === path ||
          file.path?.replaceAll('\\', '/').endsWith('/' + path.replaceAll('\\', '/'))
      )
      .sort((a, b) => a.version - b.version)
    if (!files.length) throw new Error('Для файла нет версии в истории этой сессии')
    before = files[0].content
  }
  return { path, source, before, after, patch: createTwoFilesPatch(path, path, before, after) }
}

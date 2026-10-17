import { readFile, readdir, stat } from 'node:fs/promises'
import { basename, extname, join } from 'node:path'
import ignore from 'ignore'
import { createTwoFilesPatch } from 'diff'
import { resolveProjectPath } from './paths'
import { gitFileDiff } from './git'
export async function listFiles(root: string, path = ''): Promise<any[]> {
  const absolute = await resolveProjectPath(root, path)
  const entries = await readdir(absolute, { withFileTypes: true })
  const rules = ignore()
  for (const filename of ['.gitignore', '.primeignore']) {
    try {
      rules.add(await readFile(await resolveProjectPath(root, filename), 'utf8'))
    } catch (error: any) {
      if (error.code !== 'ENOENT') throw error
    }
  }
  return entries
    .filter(
      (entry) =>
        !['.git', 'node_modules', '.superpowers', '.env', '.env.local'].includes(entry.name) &&
        !rules.ignores(
          join(path, entry.name).replaceAll('\\', '/') + (entry.isDirectory() ? '/' : '')
        )
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
  if (source === 'git') return gitFileDiff(root, path)
  let after = ''
  try {
    after = (await readProjectFile(root, path)).content || ''
  } catch (error: any) {
    if (error.code !== 'ENOENT') throw error
  }
  const files = history
    .filter(
      (file) =>
        file.path === path ||
        file.path?.replaceAll('\\', '/').endsWith('/' + path.replaceAll('\\', '/'))
    )
    .sort((a, b) => a.version - b.version)
  if (!files.length) throw new Error('Для файла нет версии в истории этой сессии')
  const before = files[0].content
  return { path, source, before, after, patch: createTwoFilesPatch(path, path, before, after) }
}

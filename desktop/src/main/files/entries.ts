import { mkdir, realpath, stat, writeFile } from 'node:fs/promises'
import { basename, dirname } from 'node:path'
import { pathToFileURL } from 'node:url'
import { resolveProjectPath } from './paths'

export type EntryKind = 'file' | 'folder'

const invalidName = /[<>:"|?*\u0000-\u001f]/

export async function createProjectEntry(root: string, path: string, kind: EntryKind) {
  const name = basename(path)
  if (!name || name === '.' || name === '..' || invalidName.test(name))
    throw new Error('Недопустимое имя')
  const absolute = await resolveProjectPath(root, path, true)
  const exists = await stat(absolute).then(
    () => true,
    () => false
  )
  if (exists) throw new Error('Файл или папка с таким именем уже существует')
  if (kind === 'folder') await mkdir(absolute, { recursive: true })
  else {
    await mkdir(dirname(absolute), { recursive: true })
    await writeFile(absolute, '', { flag: 'wx' })
  }
  return { path }
}

export async function removableProjectEntry(root: string, path: string) {
  const absolute = await resolveProjectPath(root, path)
  if (absolute === (await realpath(root))) throw new Error('Корень проекта удалить нельзя')
  return absolute
}

export async function editorUrl(root: string, path: string) {
  const absolute = await resolveProjectPath(root, path)
  return 'vscode://file' + pathToFileURL(absolute).pathname
}

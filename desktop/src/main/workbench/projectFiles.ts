import { posix } from 'node:path'
import { createTwoFilesPatch } from 'diff'
import type { RemoteConnection } from '../remote/connection'
import { remotePath } from '../remote/security'
import { listFiles, readProjectFile, fileDiff } from '../files/browser'
import { saveProjectFile } from '../files/writer'
import { gitChanges } from '../files/git'
import { createProjectEntry, editorUrl, removableProjectEntry } from '../files/entries'
import { remoteErrorSource } from '../../shared/remoteErrors'

export type ProjectTarget = { root: string; remote?: RemoteConnection }

const invalidName = /[<>:"|?*\u0000-\u001f]/

async function guarded<T>(target: ProjectTarget, operation: () => Promise<T>, translate: (text: string) => string): Promise<T> {
  try {
    return await operation()
  } catch (error) {
    const source = target.remote ? remoteErrorSource(error) : undefined
    if (source) throw new Error(translate(source))
    throw error
  }
}

async function remoteSessionDiff(connection: RemoteConnection, path: string, history: any[]) {
  let after = ''
  try {
    after = (await connection.readFile(path)).content ?? ''
  } catch (error: any) {
    if (![2, 'ENOENT'].includes(error.code)) throw error
  }
  const files = history
    .filter((file) => file.path === path || file.path?.endsWith('/' + path))
    .sort((a, b) => a.version - b.version)
  if (!files.length) throw new Error('Для файла нет версии в истории этой сессии')
  const before = files[0].content
  return { path, source: 'session', before, after, patch: createTwoFilesPatch(path, path, before, after) }
}

export function projectFiles(target: ProjectTarget, translate: (text: string) => string) {
  const remote = target.remote
  const run = <T>(operation: () => Promise<T>) => guarded(target, operation, translate)
  return {
    list: (path: string) => run(() => (remote ? remote.list(path) : listFiles(target.root, path))),
    read: (path: string) => run(() => (remote ? remote.readFile(path) : readProjectFile(target.root, path))),
    save: (path: string, content: string, revision: string) =>
      run(async () => {
        if (!remote) return saveProjectFile(target.root, path, content, revision)
        const saved = await remote.saveFile(path, content, revision)
        return { revision: saved.revision }
      }),
    changes: () => run(() => (remote ? remote.gitChanges() : gitChanges(target.root))),
    diff: (path: string, source: 'session' | 'git', history: any[]) =>
      run(() => {
        if (!remote) return fileDiff(target.root, path, source, history)
        return source === 'git' ? remote.gitDiff(path) : remoteSessionDiff(remote, path, history)
      }),
    create: (path: string, kind: 'file' | 'folder') =>
      run(async () => {
        if (!remote) return createProjectEntry(target.root, path, kind)
        const name = posix.basename(path)
        if (!name || name === '.' || name === '..' || invalidName.test(name))
          throw new Error('Недопустимое имя')
        await remote.createEntry(path, kind === 'folder')
        return { path }
      }),
    remove: (path: string, trash: (absolute: string) => Promise<void>) =>
      run(async () => {
        if (remote) return remote.removeEntry(path)
        await trash(await removableProjectEntry(target.root, path))
      }),
    editorUrl: (path: string) =>
      run(async () => {
        if (!remote) return editorUrl(target.root, path)
        const absolute = remotePath(remote.root, path)
        const { user, host, port } = remote.profile
        if (port !== 22) throw new Error('Откройте сервер в VS Code через SSH-профиль с портом 22')
        return `vscode://vscode-remote/ssh-remote+${encodeURIComponent(`${user}@${host}`)}${absolute
          .split('/')
          .map(encodeURIComponent)
          .join('/')}`
      })
  }
}

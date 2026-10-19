import { randomUUID } from 'node:crypto'
import { lstatSync, watch, type FSWatcher } from 'node:fs'
import { readFile, realpath } from 'node:fs/promises'
import { join } from 'node:path'
import ignore, { type Ignore } from 'ignore'

type WatchedProject = {
  watcher?: FSWatcher
  revision: number
  instance: string
  rules: Ignore
  loading: Promise<void>
  error?: Error
}

export class ProjectChanges {
  private readonly projects = new Map<string, Promise<WatchedProject>>()
  private closed = false

  async fingerprint(root: string): Promise<string> {
    if (this.closed) throw new Error('Project monitor is closed')
    const canonical = await realpath(root)
    const key = process.platform === 'win32' ? canonical.toLowerCase() : canonical
    let pending = this.projects.get(key)
    if (!pending) {
      pending = this.open(canonical)
      this.projects.set(key, pending)
      void pending.catch(() => this.projects.delete(key))
    }
    const project = await pending
    await project.loading
    if (this.closed) throw new Error('Project monitor is closed')
    if (project.error) throw project.error
    return `${project.instance}:${project.revision}`
  }

  dispose(): void {
    this.closed = true
    for (const pending of this.projects.values())
      void pending.then((project) => project.watcher?.close()).catch(() => {})
    this.projects.clear()
  }

  private async open(root: string): Promise<WatchedProject> {
    const project: WatchedProject = {
      revision: 0,
      instance: randomUUID(),
      rules: ignore(),
      loading: Promise.resolve()
    }
    const reload = async () => {
      const rules = ignore()
      for (const filename of ['.gitignore', '.primeignore']) {
        try {
          rules.add(await readFile(join(root, filename), 'utf8'))
        } catch (error) {
          if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error
        }
      }
      project.rules = rules
      project.error = undefined
    }
    await reload()
    if (this.closed) throw new Error('Project monitor is closed')
    project.watcher = watch(root, { recursive: true }, (_event, filename) => {
      if (!filename) {
        project.revision++
        return
      }
      const path = filename.toString().replaceAll('\\', '/')
      if (path === '.gitignore' || path === '.primeignore') {
        project.revision++
        project.loading = project.loading
          .catch(() => {})
          .then(reload)
          .catch((error) => {
            project.error = error
          })
        return
      }
      if (
        path
          .split('/')
          .some((part) => ['.git', '.prime', 'node_modules', '.superpowers'].includes(part))
      )
        return
      let directory = false
      try {
        directory = lstatSync(join(root, path)).isDirectory()
      } catch {}
      if (!project.rules.ignores(path + (directory ? '/' : ''))) project.revision++
    })
    project.watcher.on('error', (error) => {
      project.error = error
    })
    project.watcher.unref()
    return project
  }
}

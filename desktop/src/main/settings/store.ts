import { mkdir, readFile, rename, writeFile } from 'node:fs/promises'
import { dirname } from 'node:path'
import type { Preferences } from '../../shared/contracts'
const defaults: Preferences = {
  theme: 'dark',
  language: 'ru',
  recent: [],
  pinned: [],
  drafts: {},
  sidebarWidth: 256,
  inspectorWidth: 350
}
export class SettingsStore {
  private value: Preferences = structuredClone(defaults)
  private writes = Promise.resolve()
  constructor(private path: string) {}
  async load(): Promise<void> {
    try {
      const saved = JSON.parse(await readFile(this.path, 'utf8'))
      this.value = { ...defaults, ...saved.preferences }
    } catch {}
  }
  get(): Preferences {
    return structuredClone(this.value)
  }
  async update(patch: Partial<Preferences>): Promise<Preferences> {
    this.value = { ...this.value, ...patch, drafts: { ...this.value.drafts, ...patch.drafts } }
    const content = JSON.stringify({ version: 1, preferences: this.value }, null, 2)
    const persist = this.writes
      .catch(() => {})
      .then(async () => {
        await mkdir(dirname(this.path), { recursive: true })
        await writeFile(this.path + '.tmp', content)
        await rename(this.path + '.tmp', this.path)
      })
    this.writes = persist
    await persist
    return this.get()
  }
}

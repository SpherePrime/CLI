import { mkdir, readdir, readFile, rm, stat, writeFile } from 'node:fs/promises'
import { basename, extname, join } from 'node:path'
import { speechModelDirectory } from './assets'
import type { SpeechVoice } from '../../shared/speech'

export const builtInVoices: SpeechVoice[] = [
  { id: 'F1', name: 'Женский 1', custom: false },
  { id: 'F2', name: 'Женский 2', custom: false },
  { id: 'M1', name: 'Мужской 1', custom: false },
  { id: 'M2', name: 'Мужской 2', custom: false }
]

const directory = () => join(speechModelDirectory(), '..', 'voices')

function validStyle(value: any) {
  const tensor = (entry: any) =>
    entry &&
    Array.isArray(entry.dims) &&
    entry.dims.length === 3 &&
    entry.dims[0] === 1 &&
    Array.isArray(entry.data) &&
    entry.data.flat(Infinity).length === entry.dims[1] * entry.dims[2] &&
    entry.data.flat(Infinity).every((item: unknown) => typeof item === 'number' && Number.isFinite(item))
  return tensor(value?.style_ttl) && tensor(value?.style_dp)
}

export function slugify(name: string) {
  return (
    name
      .toLowerCase()
      .replace(/[^a-z0-9а-яё_-]+/gi, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 40)
      .replace(/[^a-z0-9_-]/g, (char) => char.charCodeAt(0).toString(36)) || 'voice'
  )
}

export async function listVoices(): Promise<SpeechVoice[]> {
  const custom: SpeechVoice[] = []
  try {
    for (const file of await readdir(directory())) {
      if (extname(file) !== '.json') continue
      const id = basename(file, '.json')
      let name = id
      try {
        name = JSON.parse(await readFile(join(directory(), `${id}.meta`), 'utf8')).name || id
      } catch {}
      custom.push({ id: `custom:${id}`, name, custom: true })
    }
  } catch {}
  return [...builtInVoices, ...custom]
}

export async function importVoice(path: string): Promise<SpeechVoice> {
  if ((await stat(path)).size > 5 * 1024 * 1024) throw new Error('Файл голоса больше 5 МБ')
  const value = JSON.parse(await readFile(path, 'utf8'))
  if (!validStyle(value)) throw new Error('Это не стиль голоса Supertonic (нужны style_ttl и style_dp)')
  const name = basename(path, extname(path)).slice(0, 60)
  const id = slugify(name)
  await mkdir(directory(), { recursive: true })
  await writeFile(join(directory(), `${id}.json`), JSON.stringify({ style_ttl: value.style_ttl, style_dp: value.style_dp }))
  await writeFile(join(directory(), `${id}.meta`), JSON.stringify({ name }))
  return { id: `custom:${id}`, name, custom: true }
}

export async function removeVoice(id: string) {
  const match = /^custom:([a-z0-9_-]{1,80})$/.exec(id)
  if (!match) throw new Error('Встроенный голос удалить нельзя')
  await rm(join(directory(), `${match[1]}.json`), { force: true })
  await rm(join(directory(), `${match[1]}.meta`), { force: true })
}

export async function voiceExists(id: string) {
  return (await listVoices()).some((voice) => voice.id === id)
}

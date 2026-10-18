import { createHash, randomUUID } from 'node:crypto'
import { readFile, rename, rm, stat, writeFile } from 'node:fs/promises'
import { resolveProjectPath } from './paths'
const saves = new Map<string, Promise<unknown>>()
const revision = (bytes: Buffer) => createHash('sha256').update(bytes).digest('hex')
export async function saveProjectFile(
  root: string,
  path: string,
  content: string,
  expected: string
) {
  const absolute = await resolveProjectPath(root, path)
  const previous = saves.get(absolute) || Promise.resolve()
  const pending = previous
    .catch(() => {})
    .then(async () => {
      const original = await readFile(absolute)
      if (revision(original) !== expected)
        throw new Error('Файл изменён на диске. Перечитайте его перед сохранением.')
      if (original.includes(0)) throw new Error('Бинарный файл нельзя редактировать')
      const bom = original.subarray(0, 3).equals(Buffer.from([239, 187, 191]))
      const bytes = Buffer.from(
        (bom && !content.startsWith('\uFEFF') ? '\uFEFF' : '') + content,
        'utf8'
      )
      if (bytes.length > 2 * 1024 * 1024) throw new Error('Файл больше 2 MiB')
      const temporary = absolute + '.prime-desktop-' + randomUUID() + '.tmp'
      try {
        await writeFile(temporary, bytes, { flag: 'wx', mode: (await stat(absolute)).mode })
        if (revision(await readFile(absolute)) !== expected)
          throw new Error('Файл изменён на диске. Перечитайте его перед сохранением.')
        await rename(temporary, absolute)
        return { revision: revision(bytes) }
      } finally {
        await rm(temporary, { force: true }).catch(() => {})
      }
    })
  saves.set(absolute, pending)
  try {
    return await pending
  } finally {
    if (saves.get(absolute) === pending) saves.delete(absolute)
  }
}

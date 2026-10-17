import { realpath } from 'node:fs/promises'
import { isAbsolute, relative, resolve, sep } from 'node:path'
export async function resolveProjectPath(root: string, path = ''): Promise<string> {
  const base = await realpath(root)
  const requested = resolve(base, path)
  const rel = relative(base, requested)
  if (rel === '..' || rel.startsWith(`..${sep}`) || isAbsolute(rel))
    throw new Error('Путь находится вне проекта')
  const actual = await realpath(requested)
  const actualRelative = relative(base, actual)
  if (
    actualRelative === '..' ||
    actualRelative.startsWith(`..${sep}`) ||
    isAbsolute(actualRelative)
  )
    throw new Error('Ссылка ведёт за пределы проекта')
  return actual
}

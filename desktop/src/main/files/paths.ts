import { realpath } from 'node:fs/promises'
import { dirname, isAbsolute, relative, resolve, sep } from 'node:path'
export async function resolveProjectPath(
  root: string,
  path = '',
  allowMissing = false
): Promise<string> {
  const base = await realpath(root)
  const requested = resolve(base, path)
  const rel = relative(base, requested)
  if (rel === '..' || rel.startsWith(`..${sep}`) || isAbsolute(rel))
    throw new Error('Путь находится вне проекта')
  let candidate = requested
  let actual: string
  while (true) {
    try {
      actual = await realpath(candidate)
      break
    } catch (error: any) {
      if (!allowMissing || error.code !== 'ENOENT' || candidate === dirname(candidate)) throw error
      candidate = dirname(candidate)
    }
  }
  const actualRelative = relative(base, actual)
  if (
    actualRelative === '..' ||
    actualRelative.startsWith(`..${sep}`) ||
    isAbsolute(actualRelative)
  )
    throw new Error('Ссылка ведёт за пределы проекта')
  return candidate === requested ? actual : requested
}

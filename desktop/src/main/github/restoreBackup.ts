import type { CommandRunner } from './runner'

export async function backupRestoreChanges(runner: CommandRunner, sha: string) {
  const status = await runner('git', ['status', '--porcelain', '--untracked-files=all', '--', '.'])
  if (!status.trim()) return undefined
  await runner('git', [
    'stash',
    'push',
    '--include-untracked',
    '--message',
    `Prime restore backup before ${sha.slice(0, 12)}`,
    '--',
    '.'
  ])
  const backup = (await runner('git', ['rev-parse', '--verify', 'refs/stash'])).trim()
  if ((await runner('git', ['status', '--porcelain', '--untracked-files=all', '--', '.'])).trim())
    throw new Error(
      `Локальные изменения сохранены в stash ${backup}, но рабочее дерево осталось изменённым. Восстановление отменено.`
    )
  return backup
}

export async function recoverRestoreChanges(runner: CommandRunner, backup: string | undefined) {
  if (!backup) return
  try {
    await runner('git', ['stash', 'apply', '--index', backup])
  } catch (failure) {
    throw new Error(
      `Локальные изменения сохранены в stash ${backup}. Не удалось применить резервную копию: ${String(failure)}`
    )
  }
}

export async function guardIgnoredRestoreFiles(runner: CommandRunner, sha: string) {
  const [ignored, target, ignoreCase] = await Promise.all([
    runner('git', [
      'ls-files',
      '--others',
      '--ignored',
      '--exclude-standard',
      '--directory',
      '-z',
      '--',
      '.'
    ]),
    runner('git', ['ls-tree', '-rz', '--name-only', sha, '--', '.']),
    runner('git', ['config', '--get', 'core.ignorecase']).then(
      (value) => value.trim() === 'true',
      () => false
    )
  ])
  const normalize = (path: string) => {
    const normalized = path.replace(/\/$/, '')
    return ignoreCase ? normalized.toLowerCase() : normalized
  }
  const trackedPaths = target.split('\0').filter(Boolean).map(normalize)
  for (const ignoredPath of ignored.split('\0').filter(Boolean)) {
    const path = normalize(ignoredPath)
    if (
      trackedPaths.some(
        (tracked) =>
          tracked === path || tracked.startsWith(`${path}/`) || path.startsWith(`${tracked}/`)
      )
    )
      throw new Error(
        `Восстановление может заменить игнорируемый файл или каталог «${ignoredPath}». Переместите его в безопасное место и повторите операцию.`
      )
  }
}

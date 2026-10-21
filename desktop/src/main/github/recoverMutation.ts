import type { CommandRunner } from './runner'
import { gitHead as head } from './gitMutation'
import { recoverRestoreChanges } from './restoreBackup'

export async function recoverGitMutation(
  runner: CommandRunner,
  previousHead: string,
  backupStash: string | undefined,
  treeChanged: boolean,
  error: unknown
): Promise<never> {
  const failedHead = await head(runner).catch(() => '')
  if (failedHead !== previousHead)
    throw new Error(
      `${String(error)}\nHEAD изменился во время восстановления. Автоматический откат и применение stash отменены.${treeChanged ? ' Рабочее дерево может содержать восстановленные файлы; проверьте git status.' : ''}${backupStash ? `\nЛокальные изменения сохранены в stash ${backupStash}. Для ручного восстановления: git stash apply --index ${backupStash}` : ''}`
    )
  if (treeChanged) {
    try {
      await runner('git', [
        'restore',
        `--source=${previousHead}`,
        '--staged',
        '--worktree',
        '--',
        '.'
      ])
    } catch (recoveryError) {
      throw new Error(
        `${String(error)}\nНе удалось восстановить рабочее дерево: ${String(recoveryError)}${backupStash ? `\nЛокальные изменения сохранены в stash ${backupStash}.` : ''}`
      )
    }
  }
  try {
    await recoverRestoreChanges(runner, backupStash)
  } catch (recoveryError) {
    throw new Error(`${String(error)}\n${String(recoveryError)}`)
  }
  throw error
}

import type { CommandRunner } from './runner'
import type { GitRestoreResult } from '../../shared/gitHistory'
import { gitHead, guardGitMutation, serializeGitMutation } from './gitMutation'
import { backupRestoreChanges, guardIgnoredRestoreFiles } from './restoreBackup'
import { recoverGitMutation } from './recoverMutation'

async function revert(
  runner: CommandRunner,
  sha: string,
  expectedHead: string
): Promise<GitRestoreResult> {
  const previousHead = await guardGitMutation(runner, sha, expectedHead)
  const parents = (await runner('git', ['show', '--no-patch', '--format=%P', sha]))
    .trim()
    .split(/\s+/)
    .filter(Boolean)
  const parent = parents[0]
  const patch = parent
    ? await runner('git', [
        'diff',
        '--no-renames',
        '--binary',
        '--full-index',
        '--no-ext-diff',
        '--no-textconv',
        sha,
        parent,
        '--',
        '.'
      ])
    : await runner('git', [
        'diff-tree',
        '--no-renames',
        '--root',
        '--no-commit-id',
        '-p',
        '--binary',
        '--full-index',
        '--no-ext-diff',
        '--no-textconv',
        '-R',
        sha,
        '--',
        '.'
      ])
  if (!patch.trim())
    return { previousHead, currentHead: previousHead, restoredCommit: sha, changed: false }
  const paths = parent
    ? await runner('git', [
        'diff',
        '--no-renames',
        '--name-only',
        '--relative',
        '-z',
        sha,
        parent,
        '--',
        '.'
      ])
    : await runner('git', [
        'diff-tree',
        '--no-renames',
        '--root',
        '--no-commit-id',
        '--name-only',
        '--relative',
        '-r',
        '-z',
        sha,
        '--',
        '.'
      ])
  await guardIgnoredRestoreFiles(runner, sha, paths.split('\0').filter(Boolean))
  const backupStash = await backupRestoreChanges(runner, sha)
  let treeChanged = false
  try {
    if ((await gitHead(runner)) !== previousHead)
      throw new Error('История изменилась. Обновите список коммитов.')
    if ((await runner('git', ['status', '--porcelain', '--untracked-files=all', '--', '.'])).trim())
      throw new Error(
        'Во время отмены коммита появились новые локальные изменения. Повторите операцию.'
      )
    treeChanged = true
    try {
      await runner('git', ['apply', '--3way', '--index', '--binary', '-'], patch)
    } catch (failure) {
      throw new Error(
        `Не удалось отменить изменения коммита: обнаружен конфликт с последующими изменениями. ${String(failure)}`
      )
    }
    if ((await gitHead(runner)) !== previousHead)
      throw new Error('История изменилась. Обновите список коммитов.')
    const changed = (
      await runner('git', ['diff', '--no-renames', '--cached', '--name-only', '--', '.'])
    ).trim()
    if (!changed)
      return {
        previousHead,
        currentHead: previousHead,
        restoredCommit: sha,
        changed: false,
        backupStash
      }
    await runner('git', [
      'commit',
      '--only',
      '-m',
      `Revert project changes from ${sha.slice(0, 12)}`,
      '--',
      '.'
    ])
    return {
      previousHead,
      currentHead: await gitHead(runner),
      restoredCommit: sha,
      changed: true,
      backupStash
    }
  } catch (failure) {
    return recoverGitMutation(runner, previousHead, backupStash, treeChanged, failure)
  }
}

export async function revertGitCommit(
  runner: CommandRunner,
  sha: string,
  expectedHead: string,
  workspaceKey = ''
) {
  return serializeGitMutation(runner, workspaceKey, () => revert(runner, sha, expectedHead))
}

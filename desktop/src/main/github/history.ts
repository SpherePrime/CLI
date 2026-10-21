import {
  gitHead as head,
  validateCommit,
  guardGitMutation,
  serializeGitMutation
} from './gitMutation'
import { recoverGitMutation } from './recoverMutation'
import type { CommandRunner } from './runner'
import { backupRestoreChanges, guardIgnoredRestoreFiles } from './restoreBackup'
import type {
  GitCommitDetail,
  GitHistory,
  GitHistoryCommit,
  GitRestoreResult
} from '../../shared/gitHistory'

const commitFormat = '%H%x00%P%x00%s%x00%an%x00%aI%x00%D'

function parseCommit(record: string): GitHistoryCommit {
  const [sha, parents, title, author, date, refs] = record.trim().split('\0')
  return { sha, parents: parents ? parents.split(' ') : [], title, author, date, refs: refs ?? '' }
}

export async function gitHistory(runner: CommandRunner): Promise<GitHistory> {
  const [currentHead, branch, status, output] = await Promise.all([
    head(runner),
    runner('git', ['rev-parse', '--abbrev-ref', 'HEAD']),
    runner('git', ['status', '--porcelain', '--untracked-files=all']),
    runner('git', [
      'log',
      '--all',
      'HEAD',
      '--date-order',
      '--max-count=100',
      `--format=${commitFormat}%x1e`,
      '--',
      '.'
    ])
  ])
  return {
    currentHead,
    branch: branch.trim(),
    clean: !status.trim(),
    commits: output
      .split('\x1e')
      .filter((record) => record.trim())
      .map(parseCommit)
  }
}

export async function showGitCommit(runner: CommandRunner, sha: string): Promise<GitCommitDetail> {
  await validateCommit(runner, sha)
  const [currentHead, metadata, output] = await Promise.all([
    head(runner),
    runner('git', ['show', '--no-patch', `--format=${commitFormat}`, sha]),
    runner('git', [
      'show',
      '--format=',
      '--no-ext-diff',
      '--no-textconv',
      '--stat',
      '--patch',
      sha,
      '--',
      '.'
    ])
  ])
  return {
    currentHead,
    commit: parseCommit(metadata),
    diff: output.slice(0, 300000),
    truncated: output.length > 300000
  }
}

async function restore(
  runner: CommandRunner,
  sha: string,
  expectedHead: string
): Promise<GitRestoreResult> {
  const previousHead = await guardGitMutation(runner, sha, expectedHead)
  const difference = await runner('git', ['diff', '--name-only', previousHead, sha, '--', '.'])
  await guardIgnoredRestoreFiles(runner, sha)
  const backupStash = await backupRestoreChanges(runner, sha)
  let treeChanged = false
  try {
    if ((await head(runner)) !== previousHead)
      throw new Error('История изменилась. Обновите список коммитов.')
    if (!difference.trim())
      return {
        previousHead,
        currentHead: previousHead,
        restoredCommit: sha,
        changed: false,
        backupStash
      }
    if ((await runner('git', ['status', '--porcelain', '--untracked-files=all', '--', '.'])).trim())
      throw new Error(
        'Во время восстановления появились новые локальные изменения. Повторите операцию.'
      )
    treeChanged = true
    await runner('git', ['restore', `--source=${sha}`, '--staged', '--worktree', '--', '.'])
    if ((await head(runner)) !== previousHead)
      throw new Error('История изменилась. Обновите список коммитов.')
    await runner('git', [
      'commit',
      '--only',
      '-m',
      `Restore project to ${sha.slice(0, 12)}`,
      '--',
      '.'
    ])
    return {
      previousHead,
      currentHead: await head(runner),
      restoredCommit: sha,
      changed: true,
      backupStash
    }
  } catch (error) {
    return recoverGitMutation(runner, previousHead, backupStash, treeChanged, error)
  }
}

export async function restoreGitCommit(
  runner: CommandRunner,
  sha: string,
  expectedHead: string,
  workspaceKey = ''
) {
  return serializeGitMutation(runner, workspaceKey, () => restore(runner, sha, expectedHead))
}

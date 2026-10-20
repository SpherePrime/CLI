import type { CommandRunner } from './runner'
import type {
  GitCommitDetail,
  GitHistory,
  GitHistoryCommit,
  GitRestoreResult
} from '../../shared/gitHistory'

const commitFormat = '%H%x00%P%x00%s%x00%an%x00%aI%x00%D'
const mutations = new Map<string, Promise<unknown>>()

function parseCommit(record: string): GitHistoryCommit {
  const [sha, parents, title, author, date, refs] = record.trim().split('\0')
  return { sha, parents: parents ? parents.split(' ') : [], title, author, date, refs: refs ?? '' }
}

async function head(runner: CommandRunner) {
  return (await runner('git', ['rev-parse', '--verify', 'HEAD'])).trim()
}

async function validateCommit(runner: CommandRunner, sha: string) {
  if (!/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/i.test(sha))
    throw new Error('Некорректный идентификатор коммита.')
  const resolved = (await runner('git', ['rev-parse', '--verify', `${sha}^{commit}`])).trim()
  if (resolved.toLowerCase() !== sha.toLowerCase()) throw new Error('Коммит не найден.')
  const containingRefs = await runner('git', [
    'for-each-ref',
    `--contains=${sha}`,
    '--format=%(refname)'
  ])
  if (!containingRefs.trim()) {
    await runner('git', ['merge-base', '--is-ancestor', sha, 'HEAD']).catch(() => {
      throw new Error('Коммит не принадлежит истории репозитория.')
    })
  }
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
  await validateCommit(runner, sha)
  const previousHead = await head(runner)
  if (previousHead !== expectedHead)
    throw new Error('История изменилась. Обновите список коммитов.')
  await runner('git', ['symbolic-ref', '--quiet', 'HEAD']).catch(() => {
    throw new Error('Переключитесь на ветку перед восстановлением.')
  })
  for (const marker of ['MERGE_HEAD', 'REBASE_HEAD', 'REVERT_HEAD', 'CHERRY_PICK_HEAD']) {
    const active = await runner('git', ['rev-parse', '--verify', '--quiet', marker]).then(
      () => true,
      () => false
    )
    if (active) throw new Error('Сначала завершите текущую операцию Git.')
  }
  if ((await runner('git', ['status', '--porcelain', '--untracked-files=all'])).trim()) {
    throw new Error(
      'Сохраните или уберите локальные изменения и новые файлы перед восстановлением.'
    )
  }
  const difference = await runner('git', ['diff', '--name-only', previousHead, sha, '--', '.'])
  if (!difference.trim())
    return { previousHead, currentHead: previousHead, restoredCommit: sha, changed: false }
  let treeChanged = false
  try {
    treeChanged = true
    await runner('git', ['restore', `--source=${sha}`, '--staged', '--worktree', '--', '.'])
    await runner('git', [
      'commit',
      '--only',
      '-m',
      `Restore project to ${sha.slice(0, 12)}`,
      '--',
      '.'
    ])
    return { previousHead, currentHead: await head(runner), restoredCommit: sha, changed: true }
  } catch (error) {
    if (treeChanged && (await head(runner)) === previousHead) {
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
          `${String(error)}\nНе удалось восстановить рабочее дерево: ${String(recoveryError)}`
        )
      }
    }
    throw error
  }
}

export async function restoreGitCommit(
  runner: CommandRunner,
  sha: string,
  expectedHead: string,
  workspaceKey = ''
) {
  const gitDirectory = (await runner('git', ['rev-parse', '--absolute-git-dir'])).trim()
  const key = `${workspaceKey}:${gitDirectory}`
  const previous = mutations.get(key) ?? Promise.resolve()
  const operation = previous.catch(() => undefined).then(() => restore(runner, sha, expectedHead))
  mutations.set(key, operation)
  try {
    return await operation
  } finally {
    if (mutations.get(key) === operation) mutations.delete(key)
  }
}

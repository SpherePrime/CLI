import type { CommandRunner } from './runner'

const mutations = new Map<string, Promise<unknown>>()

export async function gitHead(runner: CommandRunner) {
  return (await runner('git', ['rev-parse', '--verify', 'HEAD'])).trim()
}

export async function validateCommit(runner: CommandRunner, sha: string) {
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

export async function guardGitMutation(runner: CommandRunner, sha: string, expectedHead: string) {
  await validateCommit(runner, sha)
  const previousHead = await gitHead(runner)
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
  return previousHead
}

export async function serializeGitMutation<T>(
  runner: CommandRunner,
  workspaceKey: string,
  operation: () => Promise<T>
): Promise<T> {
  const gitDirectory = (await runner('git', ['rev-parse', '--absolute-git-dir'])).trim()
  const key = `${workspaceKey}:${gitDirectory}`
  const previous = mutations.get(key) ?? Promise.resolve()
  const pending = previous.catch(() => undefined).then(operation)
  mutations.set(key, pending)
  try {
    return await pending
  } finally {
    if (mutations.get(key) === pending) mutations.delete(key)
  }
}

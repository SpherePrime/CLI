import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, it } from 'vitest'
import { localRunner } from '../../src/main/github/runner'
import { githubOperation } from '../../src/main/github/service'

async function repository() {
  const directory = await mkdtemp(join(tmpdir(), 'prime-git-revert-'))
  const runner = localRunner(directory)
  await runner('git', ['init', '-b', 'main'])
  await runner('git', ['config', 'user.name', 'Git Test'])
  await runner('git', ['config', 'user.email', 'git@example.test'])
  await runner('git', ['config', 'commit.gpgSign', 'false'])
  await runner('git', ['config', 'core.autocrlf', 'false'])
  return { directory, runner }
}

it('undoes an older commit only within the nested project and retains later unrelated changes', async () => {
  const { directory, runner } = await repository()
  try {
    await mkdir(join(directory, 'project'))
    await writeFile(join(directory, 'project', 'file.txt'), 'initial')
    await writeFile(join(directory, 'outside.txt'), 'outside initial')
    await runner('git', ['add', '.'])
    await runner('git', ['commit', '-m', 'initial'])
    await writeFile(join(directory, 'project', 'file.txt'), 'selected change')
    await writeFile(join(directory, 'outside.txt'), 'outside selected')
    await runner('git', ['commit', '-am', 'selected'])
    const sha = (await runner('git', ['rev-parse', 'HEAD'])).trim()
    await writeFile(join(directory, 'project', 'later.txt'), 'later unrelated work')
    await runner('git', ['add', '.'])
    await runner('git', ['commit', '-m', 'later'])
    const expectedHead = (await runner('git', ['rev-parse', 'HEAD'])).trim()
    const result: any = await githubOperation(localRunner(join(directory, 'project')), {
      action: 'revertCommit',
      sha,
      expectedHead
    })
    expect(result.changed).toBe(true)
    expect(await readFile(join(directory, 'project', 'file.txt'), 'utf8')).toBe('initial')
    expect(await readFile(join(directory, 'project', 'later.txt'), 'utf8')).toBe(
      'later unrelated work'
    )
    expect(await readFile(join(directory, 'outside.txt'), 'utf8')).toBe('outside selected')
    expect((await runner('git', ['rev-parse', 'HEAD^'])).trim()).toBe(expectedHead)
    expect((await runner('git', ['status', '--porcelain'])).trim()).toBe('')
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
}, 30000)

it('aborts conflicting inversions and restores staged, unstaged and untracked local work', async () => {
  const { directory, runner } = await repository()
  try {
    await writeFile(join(directory, 'file.txt'), 'first\n')
    await runner('git', ['add', '.'])
    await runner('git', ['commit', '-m', 'initial'])
    await writeFile(join(directory, 'file.txt'), 'selected\n')
    await runner('git', ['commit', '-am', 'selected'])
    const sha = (await runner('git', ['rev-parse', 'HEAD'])).trim()
    await writeFile(join(directory, 'file.txt'), 'later conflicting work\n')
    await runner('git', ['commit', '-am', 'later'])
    const expectedHead = (await runner('git', ['rev-parse', 'HEAD'])).trim()
    await writeFile(join(directory, 'file.txt'), 'staged local work\n')
    await runner('git', ['add', '.'])
    await writeFile(join(directory, 'file.txt'), 'unstaged local work\n')
    await writeFile(join(directory, 'new.txt'), 'new local work')
    const originalStatus = await runner('git', ['status', '--porcelain'])
    await expect(
      githubOperation(runner, { action: 'revertCommit', sha, expectedHead })
    ).rejects.toThrow('конфликт')
    expect((await runner('git', ['rev-parse', 'HEAD'])).trim()).toBe(expectedHead)
    expect(await readFile(join(directory, 'file.txt'), 'utf8')).toBe('unstaged local work\n')
    expect(await runner('git', ['show', ':file.txt'])).toBe('staged local work\n')
    expect(await readFile(join(directory, 'new.txt'), 'utf8')).toBe('new local work')
    expect(await runner('git', ['status', '--porcelain'])).toBe(originalStatus)
    expect(await runner('git', ['stash', 'list'])).toContain('Prime restore backup')
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
}, 30000)

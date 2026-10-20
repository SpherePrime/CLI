import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, it } from 'vitest'
import { localRunner } from '../../src/main/github/runner'
import { githubOperation } from '../../src/main/github/service'

it('reads existing history and restores a selected tree with a new commit while protecting dirty files', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'prime-git-history-'))
  const runner = localRunner(directory)
  try {
    await runner('git', ['init', '-b', 'main'])
    await runner('git', ['config', 'user.name', 'History Test'])
    await runner('git', ['config', 'user.email', 'history@example.test'])
    await runner('git', ['config', 'commit.gpgSign', 'false'])
    for (const version of ['one', 'two', 'three']) {
      await writeFile(join(directory, 'project.txt'), version)
      await runner('git', ['add', '.'])
      await runner('git', ['commit', '-m', version])
    }
    const history: any = await githubOperation(runner, { action: 'history' })
    expect(history.commits.map((commit: any) => commit.title)).toEqual(['three', 'two', 'one'])
    const sha = history.commits[2].sha
    const preview: any = await githubOperation(runner, { action: 'showCommit', sha })
    expect(preview.diff).toContain('+one')
    await writeFile(join(directory, 'project.txt'), 'unsaved')
    await expect(
      githubOperation(runner, { action: 'restoreCommit', sha, expectedHead: preview.currentHead })
    ).rejects.toThrow('локальные изменения')
    expect(await readFile(join(directory, 'project.txt'), 'utf8')).toBe('unsaved')
    await runner('git', ['restore', '--', '.'])
    await writeFile(join(directory, 'untracked.txt'), 'keep')
    await expect(
      githubOperation(runner, { action: 'restoreCommit', sha, expectedHead: preview.currentHead })
    ).rejects.toThrow('локальные изменения')
    expect(await readFile(join(directory, 'untracked.txt'), 'utf8')).toBe('keep')
    await rm(join(directory, 'untracked.txt'))
    const result: any = await githubOperation(runner, {
      action: 'restoreCommit',
      sha,
      expectedHead: preview.currentHead
    })
    expect(result.changed).toBe(true)
    expect(await readFile(join(directory, 'project.txt'), 'utf8')).toBe('one')
    expect((await runner('git', ['rev-parse', 'HEAD^'])).trim()).toBe(history.currentHead)
    expect((await runner('git', ['rev-list', '--count', 'HEAD'])).trim()).toBe('4')
    expect((await runner('git', ['status', '--porcelain'])).trim()).toBe('')
    await expect(
      githubOperation(runner, { action: 'restoreCommit', sha, expectedHead: preview.currentHead })
    ).rejects.toThrow('История изменилась')
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
}, 30000)

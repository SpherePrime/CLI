import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, it } from 'vitest'
import { localRunner } from '../../src/main/github/runner'
import { githubOperation } from '../../src/main/github/service'

it('reads existing history and restores a selected tree with a new commit while backing up staged, unsaved and new files', async () => {
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
    await writeFile(join(directory, 'project.txt'), 'staged work')
    await runner('git', ['add', 'project.txt'])
    await writeFile(join(directory, 'project.txt'), 'unsaved work')
    await writeFile(join(directory, 'untracked.txt'), 'keep')
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
    expect(result.backupStash).toMatch(/^[a-f0-9]{40,64}$/)
    expect(await runner('git', ['show', `${result.backupStash}:project.txt`])).toBe('unsaved work')
    expect(await runner('git', ['show', `${result.backupStash}^2:project.txt`])).toBe('staged work')
    expect(await runner('git', ['show', `${result.backupStash}^3:untracked.txt`])).toBe('keep')

    await expect(
      githubOperation(runner, { action: 'restoreCommit', sha, expectedHead: preview.currentHead })
    ).rejects.toThrow('История изменилась')
    await runner('git', ['checkout', '-b', 'recover-local-work', result.previousHead])
    await runner('git', ['stash', 'apply', '--index', result.backupStash])
    expect(await readFile(join(directory, 'project.txt'), 'utf8')).toBe('unsaved work')
    expect(await runner('git', ['show', ':project.txt'])).toBe('staged work')
    expect(await readFile(join(directory, 'untracked.txt'), 'utf8')).toBe('keep')
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
}, 30000)

it('recovers staged and new files after a restore commit fails and keeps its backup', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'prime-git-history-rollback-'))
  const runner = localRunner(directory)
  try {
    await runner('git', ['init', '-b', 'main'])
    await runner('git', ['config', 'user.name', 'History Test'])
    await runner('git', ['config', 'user.email', 'history@example.test'])
    await runner('git', ['config', 'commit.gpgSign', 'false'])
    await writeFile(join(directory, 'project.txt'), 'first')
    await runner('git', ['add', '.'])
    await runner('git', ['commit', '-m', 'first'])
    const sha = (await runner('git', ['rev-parse', 'HEAD'])).trim()
    await writeFile(join(directory, 'project.txt'), 'second')
    await runner('git', ['commit', '-am', 'second'])
    const expectedHead = (await runner('git', ['rev-parse', 'HEAD'])).trim()
    await writeFile(join(directory, 'project.txt'), 'staged work')
    await runner('git', ['add', '.'])
    await writeFile(join(directory, 'project.txt'), 'unsaved work')
    await writeFile(join(directory, 'new.txt'), 'new work')
    const failCommit: typeof runner = (command, args) =>
      args[0] === 'commit'
        ? Promise.reject(new Error('test commit rejected'))
        : runner(command, args)
    await expect(
      githubOperation(failCommit, { action: 'restoreCommit', sha, expectedHead })
    ).rejects.toThrow('test commit rejected')
    expect((await runner('git', ['rev-parse', 'HEAD'])).trim()).toBe(expectedHead)
    expect(await readFile(join(directory, 'project.txt'), 'utf8')).toBe('unsaved work')
    expect(await runner('git', ['show', ':project.txt'])).toBe('staged work')
    expect(await readFile(join(directory, 'new.txt'), 'utf8')).toBe('new work')
    expect(await runner('git', ['stash', 'list'])).toContain('Prime restore backup')
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
}, 30000)

it('rejects restore collisions with ignored directories before creating a backup or changing files', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'prime-git-history-ignored-'))
  const runner = localRunner(directory)
  try {
    await runner('git', ['init', '-b', 'main'])
    await runner('git', ['config', 'user.name', 'History Test'])
    await runner('git', ['config', 'user.email', 'history@example.test'])
    await runner('git', ['config', 'commit.gpgSign', 'false'])
    await mkdir(join(directory, 'private-cache'))
    await writeFile(join(directory, 'private-cache', 'data.txt'), 'old tracked content')
    await writeFile(join(directory, 'project.txt'), 'project')
    await runner('git', ['add', '.'])
    await runner('git', ['commit', '-m', 'first'])
    const sha = (await runner('git', ['rev-parse', 'HEAD'])).trim()
    await runner('git', ['rm', '-r', 'private-cache'])
    await writeFile(join(directory, '.gitignore'), 'private-cache/\n')
    await runner('git', ['add', '.'])
    await runner('git', ['commit', '-m', 'ignore local cache'])
    const expectedHead = (await runner('git', ['rev-parse', 'HEAD'])).trim()
    await mkdir(join(directory, 'private-cache'))
    await writeFile(join(directory, 'private-cache', 'data.txt'), 'private data to keep')
    await writeFile(join(directory, 'project.txt'), 'unsaved project')
    const beforeStatus = await runner('git', ['status', '--porcelain'])
    await expect(
      githubOperation(runner, { action: 'restoreCommit', sha, expectedHead })
    ).rejects.toThrow('игнорируемый файл или каталог')
    expect(await readFile(join(directory, 'private-cache', 'data.txt'), 'utf8')).toBe(
      'private data to keep'
    )
    expect(await readFile(join(directory, 'project.txt'), 'utf8')).toBe('unsaved project')
    expect(await runner('git', ['status', '--porcelain'])).toBe(beforeStatus)
    expect((await runner('git', ['rev-parse', 'HEAD'])).trim()).toBe(expectedHead)
    expect(await runner('git', ['stash', 'list'])).toBe('')
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
}, 30000)

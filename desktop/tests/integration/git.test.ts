import { expect, it } from 'vitest'
import { mkdtemp, writeFile, rename, unlink, readFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { execFileSync } from 'node:child_process'
import { gitChanges, gitFileDiff } from '../../src/main/files/git'

it('shows modified, deleted, renamed and untracked files without changing user content', async () => {
  const root = await mkdtemp(join(tmpdir(), 'prime-git-'))
  const git = (...args: string[]) => execFileSync('git', args, { cwd: root, windowsHide: true })
  git('init')
  git('config', 'core.autocrlf', 'false')
  await writeFile(join(root, 'original.txt'), 'original content\n')
  await writeFile(join(root, 'deleted.txt'), 'deleted content\n')
  await writeFile(join(root, 'modified.txt'), 'before\n')
  git('add', '.')
  git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.test', 'commit', '-m', 'fixture')
  await rename(join(root, 'original.txt'), join(root, 'renamed file.txt'))
  git('add', '-A')
  await unlink(join(root, 'deleted.txt'))
  await writeFile(join(root, 'modified.txt'), 'after\n')
  await writeFile(join(root, 'новый файл.txt'), 'new file\n')
  const changes = await gitChanges(root)
  expect(changes.map((item) => item.path).sort()).toEqual(
    ['deleted.txt', 'modified.txt', 'renamed file.txt', 'новый файл.txt'].sort()
  )
  expect(changes.find((item) => item.path === 'renamed file.txt')?.originalPath).toBe(
    'original.txt'
  )
  expect((await gitFileDiff(root, 'renamed file.txt')).before).toBe('original content\n')
  expect((await gitFileDiff(root, 'новый файл.txt')).before).toBe('')
  expect((await gitFileDiff(root, 'deleted.txt')).after).toBe('')
  expect(await readFile(join(root, 'modified.txt'), 'utf8')).toBe('after\n')
})

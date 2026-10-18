import { expect, it } from 'vitest'
import { mkdtemp, mkdir, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { listFiles } from '../../src/main/files/browser'

it('hides ignored workspace content while honoring explicit re-inclusion', async () => {
  const root = await mkdtemp(join(tmpdir(), 'prime-files-'))
  await mkdir(join(root, 'cache'))
  await writeFile(join(root, '.gitignore'), 'cache/\n*.log\n!keep.log\n')
  await writeFile(join(root, '.primeignore'), 'private.txt\n')
  for (const name of ['private.txt', 'output.log', 'keep.log', 'hello.txt', '.env'])
    await writeFile(join(root, name), 'fixture')
  const names = (await listFiles(root)).map((item) => item.name)
  expect(names).toContain('keep.log')
  expect(names).toContain('hello.txt')
  for (const name of ['cache', 'private.txt', 'output.log', '.env'])
    expect(names).not.toContain(name)
})

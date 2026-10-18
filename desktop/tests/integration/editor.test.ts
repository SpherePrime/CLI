import { expect, it } from 'vitest'
import { mkdtemp, readFile, writeFile, symlink } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { readProjectFile } from '../../src/main/files/browser'
import { saveProjectFile } from '../../src/main/files/writer'

it('saves an edited UTF8 file and refuses to overwrite an external change', async () => {
  const root = await mkdtemp(join(tmpdir(), 'prime-editor-'))
  const path = join(root, 'код.ts')
  await writeFile(path, 'export const value = 1\r\n')
  const opened = await readProjectFile(root, 'код.ts')
  const saved = await saveProjectFile(root, 'код.ts', 'export const value = 2\r\n', opened.revision)
  expect(await readFile(path, 'utf8')).toBe('export const value = 2\r\n')
  await writeFile(path, 'external edit')
  await expect(saveProjectFile(root, 'код.ts', 'local edit', saved.revision)).rejects.toThrow(
    /изменён/
  )
  expect(await readFile(path, 'utf8')).toBe('external edit')
  await expect(saveProjectFile(root, '../outside', 'invalid', saved.revision)).rejects.toThrow()
})

it('preserves the UTF8 BOM and rejects writes through an escaping junction', async () => {
  const root = await mkdtemp(join(tmpdir(), 'prime-editor-'))
  const outside = await mkdtemp(join(tmpdir(), 'prime-editor-outside-'))
  await writeFile(join(root, 'bom.txt'), Buffer.from('\uFEFForiginal'))
  const opened = await readProjectFile(root, 'bom.txt')
  await saveProjectFile(root, 'bom.txt', 'changed', opened.revision)
  expect(await readFile(join(root, 'bom.txt'), 'utf8')).toBe('\uFEFFchanged')
  await writeFile(join(outside, 'secret.txt'), 'untouched')
  await symlink(outside, join(root, 'escape'), 'junction')
  await expect(
    saveProjectFile(root, 'escape/secret.txt', 'overwritten', opened.revision)
  ).rejects.toThrow()
  expect(await readFile(join(outside, 'secret.txt'), 'utf8')).toBe('untouched')
})

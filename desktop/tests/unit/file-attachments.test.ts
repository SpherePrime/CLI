import { expect, it } from 'vitest'
import { mkdtemp, rm, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { importFiles } from '../../src/main/attachments/files'
import type { StoredAttachment } from '../../src/shared/attachments'

it('keeps a failed file batch from retaining attachments with unreachable IDs', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'prime-attachments-'))
  const store = new Map<string, StoredAttachment>()
  try {
    const valid = join(directory, 'small.txt')
    await writeFile(valid, 'attached')
    await expect(importFiles(store, [valid, join(directory, 'missing.txt')])).rejects.toThrow()
    expect(store.size).toBe(0)
    const result = await importFiles(store, [valid])
    expect(result).toEqual([{ id: expect.any(String), name: 'small.txt', size: 8 }])
    expect(store.get(result[0].id)?.content).toBe('YXR0YWNoZWQ=')
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})

it('applies the pending attachment memory budget to file imports', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'prime-attachments-'))
  const store = new Map<string, StoredAttachment>()
  const existing = {
    file_path: 'large',
    file_name: 'large',
    mime_type: 'text/plain',
    content: 'A'.repeat(28 * 1024 * 1024)
  }
  for (let index = 0; index < 10; index++) store.set(String(index), existing)
  try {
    const valid = join(directory, 'small.txt')
    await writeFile(valid, 'attached')
    await expect(importFiles(store, [valid])).rejects.toThrow('Слишком много вложений')
    expect(store.size).toBe(10)
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})

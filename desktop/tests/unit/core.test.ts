import { describe, expect, it } from 'vitest'
import { SseDecoder } from '../../src/main/prime/sse'
import { applyMessage } from '../../src/shared/messages'
import { resolveProjectPath } from '../../src/main/files/paths'
import { mkdtemp, mkdir, symlink } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

describe('SSE transport', () => {
  it('buffers partial UTF8 and handles multiple data lines and CRLF', () => {
    const decoder = new SseDecoder()
    const bytes = Buffer.from('data: {"text":"Привет"}\r\n\r\ndata: second\n\n')
    const split = bytes.indexOf(Buffer.from('Пр')) + 1
    expect(decoder.push(bytes.subarray(0, split))).toEqual([])
    expect(decoder.push(bytes.subarray(split))).toEqual(['{"text":"Привет"}', 'second'])
  })
})

describe('message reconciliation', () => {
  it('replaces a streamed message instead of appending it twice', () => {
    const first = { id: '1', session_id: 'a', role: 'assistant', parts: [] }
    const updated = { ...first, parts: [{ type: 'text', data: { text: 'Hello' } }] }
    expect(applyMessage([first], updated)).toEqual([updated])
  })
})

describe('project boundary', () => {
  it('rejects relative traversal', async () => {
    const root = await mkdtemp(join(tmpdir(), 'prime-boundary-'))
    await expect(resolveProjectPath(root, '../secret')).rejects.toThrow()
  })
  it('rejects a junction escaping the project', async () => {
    const root = await mkdtemp(join(tmpdir(), 'prime-boundary-'))
    const outside = await mkdtemp(join(tmpdir(), 'prime-outside-'))
    await mkdir(join(root, 'inside'))
    await symlink(outside, join(root, 'escape'), 'junction')
    await expect(resolveProjectPath(root, 'escape')).rejects.toThrow()
  })
})

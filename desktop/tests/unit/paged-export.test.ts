import { expect, it, vi } from 'vitest'
import { conversationExportChunks } from '../../src/renderer/src/features/chat/pagedExport'

it('exports older unloaded text pages in order without retrieving image payloads', async () => {
  const call = vi.fn(async (_project: string, _operation: string, args: any) => ({
    messages: [
      {
        id: args.before ? 'old' : 'new',
        session_id: 's',
        role: 'user',
        parts: [{ type: 'text', data: { text: args.before ? 'Oldest message' : 'Latest message' } }]
      }
    ],
    before: args.before ? 'old' : 'new',
    has_before: !args.before
  }))
  const chunks = await conversationExportChunks('project', 's', 'Example', (text) => text, call)
  const output = chunks.join('')
  expect(output.indexOf('Oldest message')).toBeLessThan(output.indexOf('Latest message'))
  expect(output.match(/# Example/g)).toHaveLength(1)
  expect(
    call.mock.calls.every(([, operation, args]) => operation === 'messagePage' && args.textOnly)
  ).toBe(true)
})

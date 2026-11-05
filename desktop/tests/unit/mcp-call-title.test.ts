import { describe, expect, it } from 'vitest'
import { toolTitle } from '../../src/renderer/src/features/chat/toolTitle'

const t = (value: string) => value

describe('mcp_call title', () => {
  it('shows the grouped MCP tool instead of the router', () => {
    const title = toolTitle('mcp_call', { tool: 'mcp_docs_search', arguments: { query: 'x' } }, t)
    expect(title.label).toBe('MCP · docs')
    expect(title.detail).toBe('search')
  })
})

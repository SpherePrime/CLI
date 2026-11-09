import { describe, expect, it } from 'vitest'
import { toolTitle } from '../../src/renderer/src/features/chat/toolTitle'

const t = (value: string) => value

describe('mcp_call title', () => {
  it('shows the grouped MCP tool instead of the router', () => {
    const title = toolTitle('mcp_call', { tool: 'mcp_docs_search', arguments: { query: 'x' } }, t)
    expect(title.label).toBe('MCP · docs')
    expect(title.detail).toBe('search')
  })

  it('recognizes desktop area servers', () => {
    expect(toolTitle('mcp_call', { tool: 'mcp_apps_app_find', arguments: {} }, t).label).toBe(
      'Проверка приложения'
    )
    expect(toolTitle('mcp_terminal_terminal_list', {}, t).label).toBe('Терминал')
    expect(toolTitle('mcp_prime-desktop-git_git_history', {}, t).label).toBe('Git')
    expect(toolTitle('mcp_browser_browser_click', {}, t).label).toBe('Браузер')
  })
})

describe('browser screenshot title', () => {
  it('names the capture mode', () => {
    expect(toolTitle('mcp_browser_browser_screenshot', {}, t).detail).toBe('Снимок страницы')
    expect(toolTitle('mcp_browser_browser_screenshot', { fullPage: true }, t).detail).toBe('Снимок всей страницы')
    expect(toolTitle('mcp_browser_browser_screenshot', { ref: 4 }, t).detail).toBe('Снимок элемента')
  })
})

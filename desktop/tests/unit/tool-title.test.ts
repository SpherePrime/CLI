import { expect, it } from 'vitest'
import { toolTitle } from '../../src/renderer/src/features/chat/toolTitle'

const t = (key: string) => key

it('labels SSH actions by their purpose through every built-in bridge name', () => {
  for (const bridge of ['browser', 'prime-desktop', 'prime-desktop-2']) {
    expect(toolTitle(`mcp_${bridge}_ssh_profiles`, {}, t)).toMatchObject({
      label: 'SSH',
      detail: 'Список серверов'
    })
    expect(toolTitle(`mcp_${bridge}_ssh_exec`, { command: 'uname -a' }, t)).toMatchObject({
      label: 'SSH',
      detail: 'Выполняет команду · uname -a'
    })
  }
})

it('recognizes Git and other built-in tools while preserving external MCP identity', () => {
  expect(toolTitle('mcp_browser_git_history', {}, t).label).toBe('Git')
  expect(toolTitle('mcp_prime-desktop_terminal_read', {}, t).label).toBe('Терминал')
  expect(toolTitle('ssh_read_file', { path: '/srv/app.ts' }, t)).toMatchObject({
    label: 'SSH',
    detail: 'Читает файл · /srv/app.ts'
  })
  expect(toolTitle('mcp_external_ssh_profiles', {}, t)).toMatchObject({
    label: 'MCP · external',
    detail: 'ssh_profiles'
  })
  expect(toolTitle('mcp_browser_unknown_tool', {}, t)).toMatchObject({
    label: 'Prime Desktop',
    detail: 'unknown tool'
  })
})

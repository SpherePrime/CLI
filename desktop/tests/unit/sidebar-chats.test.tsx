import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it, vi } from 'vitest'
import { SidebarChats } from '../../src/renderer/src/features/projects/SidebarChats'
import type { Workspace } from '../../src/renderer/src/app/useWorkspace'

function sidebar(personalCount = 1, pinned: string[] = []) {
  const personal = { id: 'personal', kind: 'chat', path: '/personal', connection: 'ready' }
  const project = { id: 'project', kind: 'local', path: '/repo', connection: 'ready' }
  const sessions = Array.from({ length: personalCount }, (_, index) => ({
    id: `chat-${index}`,
    title: `Personal ${index}`,
    updated_at: index
  }))
  const workspace = {
    projects: [personal, project],
    project: personal,
    sessions,
    session: sessions[0],
    busy: true,
    preferences: { recent: ['/repo'], pinned, projectOrder: [] },
    info: { runtime: 'ready' },
    savePreferences: vi.fn()
  } as unknown as Workspace
  return renderToStaticMarkup(<SidebarChats workspace={workspace} onRemote={() => {}} />)
}

it('shows a labeled working indicator for the active chat immediately', () => {
  const html = sidebar()
  expect(html).toContain('aria-label="Задача выполняется"')
  expect(html).not.toContain('dot pulse')
})

it('lists all personal chats only in recent chats', () => {
  const html = sidebar(10)
  const recent = html.indexOf('НЕДАВНИЕ')
  expect(recent).toBeGreaterThan(0)
  expect(html.slice(0, recent)).not.toContain('Personal 0')
  expect(html.match(/class="session-row recent-chat/g)).toHaveLength(10)
})

it('keeps pinned personal chats marked in recent chats', () => {
  expect(sidebar(1, ['chat-0'])).toContain('lucide-pin')
})

it('keeps pinned personal chats above newer unpinned chats', () => {
  const html = sidebar(3, ['chat-0'])
  expect(html.indexOf('title="Personal 0"')).toBeLessThan(html.indexOf('title="Personal 2"'))
})

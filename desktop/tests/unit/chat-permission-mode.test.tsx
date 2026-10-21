import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, expect, it, vi } from 'vitest'
import { useChatPermissions } from '../../src/renderer/src/features/chat/useChatPermissions'
import type { Preferences } from '../../src/shared/contracts'

afterEach(() => vi.unstubAllGlobals())

it('sending while a mode change is pending uses the newly chosen mode', async () => {
  let release!: () => void
  const delayed = new Promise<void>((resolve) => {
    release = resolve
  })
  const call = vi.fn(async (_project: string, _operation: string, _args: unknown) => delayed)
  const save = vi.fn(async (_patch: Partial<Preferences>) => {})
  vi.stubGlobal('window', { prime: { call } })
  let permissions!: ReturnType<typeof useChatPermissions>
  function Probe() {
    permissions = useChatPermissions({
      projectId: 'project',
      sessionId: 'chat',
      preferences: {
        theme: 'dark', language: 'ru', recent: [], pinned: [], drafts: {},
        sidebarWidth: 256, inspectorWidth: 350, skipBySession: { chat: false }
      },
      savePreferences: save,
      onError: vi.fn()
    })
    return null
  }
  renderToStaticMarkup(<Probe />)
  const changing = permissions.setSkip(true)
  const sending = permissions.prepareSession('project', 'chat', false)
  release()
  await Promise.all([changing, sending])
  expect(call.mock.calls).toEqual([
    ['project', 'skip', { sessionId: 'chat', skip: true }],
    ['project', 'skip', { sessionId: 'chat', skip: true }]
  ])
  expect(save.mock.calls.every(([patch]) => patch.skipBySession?.chat === true)).toBe(true)
})

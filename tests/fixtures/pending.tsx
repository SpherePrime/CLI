import React from 'react'
import { createRoot } from 'react-dom/client'
import { useWorkspace } from '../../src/renderer/src/app/useWorkspace'
import { Approvals } from '../../src/renderer/src/features/approvals/Approvals'
import '../../src/renderer/src/styles/global.css'
const listeners = new Set<(event: any) => void>()
let permissions: any[] = []
const project = { id: 'workspace', path: 'C:/fixture', connection: 'ready' }
const session = { id: 'session', title: 'Fixture' }
const preferences = { theme: 'dark', language: 'ru', recent: [], pinned: [], drafts: {} }
window.prime = {
  info: async () => ({ runtime: 'ready' }),
  preferences: async () => preferences,
  projects: async () => [project],
  onEvent: (listener: any) => {
    listeners.add(listener)
    return () => listeners.delete(listener)
  },
  call: async (_id: string, operation: string) => {
    if (operation === 'sessions') return [session]
    if (operation === 'agent') return { is_ready: true }
    if (operation === 'pending') return { permissions: [...permissions] }
    if (['messages', 'queue', 'skills', 'channelMessages'].includes(operation)) return []
    return {}
  }
} as any
function Pending() {
  const w = useWorkspace()
  const emit = (type: string, data: any) =>
    listeners.forEach((listener) => listener({ type, workspaceId: project.id, payload: { data } }))
  return (
    <>
      <div>{w.session ? 'Workspace ready' : 'Loading'}</div>
      <button
        onClick={async () => {
          emit('permission_notification', { tool_call_id: 'tool', granted: false, denied: false })
          permissions = [
            {
              id: 'request',
              session_id: session.id,
              tool_call_id: 'tool',
              tool_name: 'write',
              description: 'Permission stays pending'
            }
          ]
          emit('permission_request', permissions[0])
          await w.refresh()
        }}
      >
        Request permission
      </button>
      <button
        onClick={async () => {
          permissions = []
          emit('permission_notification', { tool_call_id: 'tool', granted: true, denied: false })
          await w.refresh()
        }}
      >
        Resolve permission
      </button>
      <Approvals workspace={w} />
    </>
  )
}
createRoot(document.getElementById('root')!).render(<Pending />)

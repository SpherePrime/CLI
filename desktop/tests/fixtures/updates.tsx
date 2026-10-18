import React, { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { UpdateScreen } from '../../src/renderer/src/features/updates/UpdateScreen'
import '../../src/renderer/src/styles/global.css'
import '../../src/renderer/src/styles/brand.css'

const listeners = new Set<(event: any) => void>()
const calls: string[] = []
Object.assign(window, {
  fixtureCalls: calls,
  prime: {
    info: async () => ({ update: { state: 'idle' } }),
    onEvent: (listener: (event: any) => void) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    update: async (action: string) => {
      calls.push(action)
      const result = {
        state: action === 'check' ? 'available' : 'installing',
        version: '0.4.52',
        percent: 0
      }
      listeners.forEach((listener) => listener({ type: 'update', payload: result }))
      return result
    }
  }
})
window.addEventListener('fixture:update', ((event: CustomEvent) =>
  listeners.forEach((listener) =>
    listener({ type: 'update', payload: event.detail })
  )) as EventListener)
function Fixture() {
  const [ready, setReady] = useState(false)
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    const start = () => setReady(true)
    const work = ((event: CustomEvent) => setBusy(event.detail)) as EventListener
    window.addEventListener('fixture:ready', start)
    window.addEventListener('fixture:busy', work)
    return () => {
      window.removeEventListener('fixture:ready', start)
      window.removeEventListener('fixture:busy', work)
    }
  }, [])
  return (
    <>
      <p>Workspace</p>
      <UpdateScreen ready={ready} busy={busy} />
    </>
  )
}
createRoot(document.getElementById('root')!).render(<Fixture />)

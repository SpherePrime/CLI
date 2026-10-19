import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import type { DesktopEvent } from '../../src/shared/contracts'
import type { TerminalRequest, TerminalResponse } from '../../src/shared/terminal'
import { TerminalPanel } from '../../src/renderer/src/features/terminal/TerminalPanel'
import { I18nProvider } from '../../src/renderer/src/i18n'
import '../../src/renderer/src/styles/global.css'
import '../../src/renderer/src/styles/brand.css'

declare global {
  interface Window {
    terminalFixture: {
      api: (request: TerminalRequest) => Promise<TerminalResponse>
      writes: () => Promise<string[]>
      subscribe: (listener: (event: DesktopEvent) => void) => () => void
    }
  }
}

function Fixture() {
  const [visible, setVisible] = useState(true)
  const [error, setError] = useState('')
  return <I18nProvider language="ru"><div style={{ display: 'flex', height: '100vh', flexDirection: 'column' }}>
    <div style={{ display: 'flex', alignItems: 'center', gap: 20, padding: '14px 18px', background: 'var(--surface)', borderBottom: '1px solid var(--border)' }}>
      <strong style={{ color: 'var(--accent)', fontSize: 14 }}>Prime Desktop</strong><span style={{ color: 'var(--secondary)', fontSize: 12 }}>CLI / workspace</span>
      <button style={{ marginLeft: 'auto', fontSize: 11 }} onClick={() => setVisible((value) => !value)}>Toggle panel</button>
    </div>
    <div style={{ display: 'flex', flex: 1, minHeight: 0 }}>
      <aside style={{ padding: 24, flex: 1, background: 'var(--sidebar)' }}><p style={{ fontSize: 11, color: 'var(--muted)' }}>WORKSPACE</p><h2 style={{ fontSize: 20, fontWeight: 500 }}>CLI</h2><p style={{ color: 'var(--secondary)', fontSize: 12 }}>src / desktop</p><p role="status">{error}</p></aside>
      <div style={{ width: 530, borderLeft: '1px solid var(--border)' }}>{visible && <TerminalPanel workspaceId="fixture" api={window.terminalFixture.api} subscribe={window.terminalFixture.subscribe} onError={setError} />}</div>
    </div>
  </div></I18nProvider>
}

createRoot(document.getElementById('root')!).render(<Fixture />)

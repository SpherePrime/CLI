import { useState } from 'react'
import { ChevronDown, ChevronRight, RefreshCw } from 'lucide-react'
import type { AppUiTarget } from '../../../../shared/appTesting'
import type { AppTestingCopy } from './copy'

type Props = {
  copy: AppTestingCopy
  windows: AppUiTarget[]
  busy: boolean
  onOpen(): void
  onRefresh(): void
  onAttach(target: AppUiTarget): void
}

export function ConnectWindow({ copy, windows, busy, onOpen, onRefresh, onAttach }: Props) {
  const [open, setOpen] = useState(false)
  return (
    <section className="app-testing-section">
      <button
        type="button"
        className="app-testing-disclosure"
        onClick={() => {
          if (!open) onOpen()
          setOpen(!open)
        }}
      >
        {open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
        {copy.connect}
      </button>
      {open && (
        <>
          <div className="app-testing-windows-toolbar">
            <button
              type="button"
              className="icon small"
              aria-label={copy.refresh}
              title={copy.refresh}
              disabled={busy}
              onClick={onRefresh}
            >
              <RefreshCw size={13} />
            </button>
          </div>
          {windows.length === 0 ? (
            <p className="app-testing-note">{copy.noWindows}</p>
          ) : (
            <ul className="app-testing-windows">
              {windows.map((target) => (
                <li key={target.hwnd}>
                  <span title={target.title}>{target.title}</span>
                  <button type="button" disabled={busy} onClick={() => onAttach(target)}>
                    {copy.connectAction}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </section>
  )
}

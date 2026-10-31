import { RefreshCw } from 'lucide-react'
import type { AppUiTarget } from '../../../../shared/appTesting'
import { appTestingCopy } from './copy'

type Props = {
  language: 'ru' | 'en'
  targets: AppUiTarget[]
  attached?: AppUiTarget | null
  busy: boolean
  onRefresh(): void
  onAttach(target: AppUiTarget, mode: 'view' | 'control'): void
  onDetach(): void
}

export function TargetList({ language, targets, attached, busy, onRefresh, onAttach, onDetach }: Props) {
  const c = appTestingCopy[language]
  return (
    <div className="app-testing-targets">
      <div className="app-testing-toolbar">
        <button
          className="icon small"
          type="button"
          aria-label={c.refresh}
          title={c.refresh}
          disabled={busy}
          onClick={onRefresh}
        >
          <RefreshCw size={14} />
        </button>
        {attached && (
          <button className="app-testing-detach" type="button" onClick={onDetach}>
            {c.detach}
          </button>
        )}
      </div>
      {targets.length === 0 ? (
        <p className="app-testing-empty" role="status">
          {c.empty}
          <br />
          {c.emptyHint}
        </p>
      ) : (
        <ul className="app-testing-list">
          {targets.map((target) => (
            <li key={target.hwnd} className={attached?.hwnd === target.hwnd ? 'active' : ''}>
              <div className="app-testing-target">
                <span className="app-testing-target-title" title={target.title}>
                  {target.title || target.hwnd}
                </span>
                <span className="app-testing-target-meta">PID {target.pid}</span>
              </div>
              <div className="app-testing-target-actions">
                <button type="button" disabled={busy} onClick={() => onAttach(target, 'view')}>
                  {c.view}
                </button>
                <button type="button" disabled={busy} onClick={() => onAttach(target, 'control')}>
                  {c.control}
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

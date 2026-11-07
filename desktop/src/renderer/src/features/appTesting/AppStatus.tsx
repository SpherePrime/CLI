import { AppWindow, Crosshair, X } from 'lucide-react'
import type { AppUiState } from '../../../../shared/appTesting'
import type { AppTestingCopy } from './copy'

type Props = {
  copy: AppTestingCopy
  state: AppUiState
  picking: boolean
  busy: boolean
  onPick(): void
  onDetach(): void
}

export function AppStatus({ copy, state, picking, busy, onPick, onDetach }: Props) {
  const { target, launched } = state
  if (!target) {
    return (
      <div className="app-testing-status idle">
        <AppWindow size={18} />
        <div>
          <strong>{copy.idle}</strong>
          <p>{copy.idleHint}</p>
        </div>
      </div>
    )
  }
  const exited = launched && !launched.running
  return (
    <div className="app-testing-status">
      <span className={`app-testing-dot ${exited ? 'off' : ''}`} />
      <div className="app-testing-status-text">
        <strong title={target.title}>{target.title}</strong>
        <span title={launched?.command}>
          {exited ? copy.stopped : launched ? copy.launchedBy : copy.attachedBy} · PID {target.pid}
          {launched ? ` · ${launched.command}` : ''}
        </span>
      </div>
      <button
        type="button"
        className={`icon small ${picking ? 'active' : ''}`}
        aria-label={copy.pick}
        aria-pressed={picking}
        title={copy.pick}
        disabled={busy && !picking}
        onClick={onPick}
      >
        <Crosshair size={15} />
      </button>
      <button
        type="button"
        className="icon small"
        aria-label={launched ? copy.stop : copy.detach}
        title={launched ? copy.stop : copy.detach}
        disabled={busy}
        onClick={onDetach}
      >
        <X size={15} />
      </button>
    </div>
  )
}

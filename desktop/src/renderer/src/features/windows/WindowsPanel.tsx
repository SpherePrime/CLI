import { useCallback, useEffect, useRef, useState } from 'react'
import {
  AppWindow,
  Camera,
  Eye,
  LoaderCircle,
  MousePointer2,
  RefreshCw,
  Search,
  ShieldCheck,
  ShieldOff
} from 'lucide-react'
import type { WindowInfo, WindowRequest, WindowState } from '../../../../shared/computer'
import { useI18n } from '../../i18n'
import { windowsCopy } from './copy'
import '../../styles/automations.css'
import '../../styles/windows.css'

type Props = {
  workspaceId: string
  sessionId?: string
  sessionTitle?: string
  onError(error: unknown): void
}
export function WindowsPanel({ workspaceId, sessionId, sessionTitle, onError }: Props) {
  const { language, locale } = useI18n()
  const c = windowsCopy[language]
  const [windows, setWindows] = useState<WindowInfo[]>([])
  const [selected, setSelected] = useState<WindowInfo>()
  const [image, setImage] = useState('')
  const [state, setState] = useState<WindowState>({ log: [] })
  const [loading, setLoading] = useState(true)
  const [capturing, setCapturing] = useState(false)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const generation = useRef(0)
  const activeWorkspace = useRef(workspaceId)
  activeWorkspace.current = workspaceId
  const call = useCallback(
    (request: WindowRequest) => window.prime.windows(workspaceId, request),
    [workspaceId]
  )
  const fail = useCallback(
    (cause: unknown) => {
      setError(cause instanceof Error ? cause.message : String(cause))
      onError(cause)
    },
    [onError]
  )
  const refresh = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [items, next] = await Promise.all([call({ action: 'list' }), call({ action: 'state' })])
      if (activeWorkspace.current !== workspaceId) return
      setWindows(items)
      setSelected((current) => items.find((item: WindowInfo) => item.hwnd === current?.hwnd))
      setState(next)
    } catch (cause) {
      fail(cause)
    } finally {
      setLoading(false)
    }
  }, [call, fail, workspaceId])
  useEffect(() => {
    generation.current++
    setSelected(undefined)
    setImage('')
    setState({ log: [] })
    void refresh()
    return window.prime.onEvent((event) => {
      if (event.type === 'windows' && event.workspaceId === workspaceId) setState(event.payload)
    })
  }, [refresh, workspaceId])
  const select = async (item: WindowInfo) => {
    const token = ++generation.current
    setSelected(item)
    setImage('')
    setCapturing(true)
    setError('')
    try {
      const result = await call({ action: 'snapshot', hwnd: item.hwnd })
      if (token === generation.current) {
        setImage(result.image)
        setSelected(result.info)
      }
    } catch (cause) {
      if (token === generation.current) fail(cause)
    } finally {
      if (token === generation.current) setCapturing(false)
    }
  }
  const update = async (request: WindowRequest) => {
    setError('')
    try {
      setState(await call(request))
    } catch (cause) {
      fail(cause)
    }
  }
  const share = state.share
  const request = state.request
  const filtered = windows.filter((item) =>
    item.title.toLocaleLowerCase().includes(query.toLocaleLowerCase())
  )
  return (
    <div className="automation-panel windows-panel" aria-label={c.title}>
      <header className="automation-header">
        <div>
          <AppWindow size={18} aria-hidden="true" />
          <h2>{c.title}</h2>
          <span className="windows-count">{windows.length}</span>
        </div>
        <button aria-label={c.refresh} disabled={loading} onClick={() => void refresh()}>
          <RefreshCw size={15} className={loading ? 'windows-spinning' : ''} />
        </button>
      </header>
      {share && (
        <div
          className={`windows-share ${share.mode === 'control' ? 'windows-control-active' : ''}`}
          role="status"
        >
          {share.mode === 'control' ? <MousePointer2 size={17} /> : <Eye size={17} />}
          <div>
            <strong>{share.mode === 'control' ? c.controlActive : c.viewActive}</strong>
            <span>
              {share.title}
              {share.sessionId !== sessionId ? ` · ${c.otherChat}` : ''}
            </span>
            <small>Ctrl+Alt+Escape</small>
          </div>
          <button className="windows-revoke" onClick={() => void update({ action: 'revoke' })}>
            <ShieldOff size={15} />
            {c.revoke}
          </button>
        </div>
      )}
      {request && (
        <section
          className="windows-request"
          aria-label={request.mode === 'control' ? c.requestControl : c.requestView}
        >
          <div>
            <ShieldCheck size={19} />
            <strong>{request.mode === 'control' ? c.requestControl : c.requestView}</strong>
          </div>
          <p>{request.title}</p>
          <small>
            {c.requestHint} {request.mode === 'control' ? c.controlHint : c.scope}
          </small>
          <div className="automation-actions">
            <button
              className="automation-primary"
              onClick={() =>
                void update({
                  action: 'share',
                  hwnd: request.hwnd,
                  sessionId: request.sessionId,
                  mode: request.mode,
                  expectedPid: request.pid,
                  expectedIdentity: request.identity
                })
              }
            >
              {request.mode === 'control' ? c.control : c.view}
            </button>
            <button onClick={() => void update({ action: 'dismiss' })}>{c.close}</button>
          </div>
        </section>
      )}
      {error && (
        <div className="windows-error" role="alert">
          {error}
        </div>
      )}
      <div className="windows-layout">
        <aside className="windows-list">
          <label className="windows-search">
            <Search size={14} />
            <input
              aria-label={c.search}
              placeholder={c.search}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </label>
          {loading && !windows.length ? (
            <div className="windows-empty">
              <LoaderCircle className="windows-spinning" size={22} />
              <p>{c.loading}</p>
            </div>
          ) : (
            filtered.map((item) => (
              <button
                key={item.hwnd}
                className={`windows-item ${selected?.hwnd === item.hwnd ? 'selected' : ''}`}
                aria-pressed={selected?.hwnd === item.hwnd}
                onClick={() => void select(item)}
              >
                <AppWindow size={17} aria-hidden="true" />
                <span>
                  <strong>{item.title}</strong>
                  <small>
                    {item.bounds.width} × {item.bounds.height} · PID {item.pid}
                  </small>
                </span>
                {share?.hwnd === item.hwnd && (
                  <Eye
                    size={13}
                    aria-label={share.mode === 'control' ? c.controlActive : c.viewActive}
                  />
                )}
              </button>
            ))
          )}
          {!loading && !filtered.length && (
            <div className="windows-empty">
              <AppWindow size={27} />
              <strong>{windows.length ? c.noMatch : c.empty}</strong>
              {!windows.length && <p>{c.emptyHint}</p>}
            </div>
          )}
        </aside>
        <section className="windows-detail">
          {!selected ? (
            <div className="windows-empty windows-intro">
              <div className="windows-intro-icon">
                <AppWindow size={30} />
              </div>
              <h3>{c.select}</h3>
              <p>{c.selectHint}</p>
            </div>
          ) : (
            <>
              <div className="windows-selected-title">
                <div>
                  <strong>{selected.title}</strong>
                  <small>
                    {c.chat}: {sessionTitle || c.newChat}
                  </small>
                </div>
                <button
                  aria-label={c.preview}
                  disabled={capturing}
                  onClick={() => void select(selected)}
                >
                  <Camera size={15} />
                </button>
              </div>
              <div className="windows-preview" aria-busy={capturing}>
                {image ? (
                  <img src={image} alt={selected.title} />
                ) : (
                  <div className="windows-empty">
                    {capturing && <LoaderCircle size={22} className="windows-spinning" />}
                    <p>{capturing ? c.snapshot : c.unavailable}</p>
                  </div>
                )}
              </div>
              <div className="automation-actions windows-grants">
                <button
                  disabled={
                    !sessionId ||
                    (share?.hwnd === selected.hwnd &&
                      share.sessionId === sessionId &&
                      share.mode === 'view')
                  }
                  title={!sessionId ? c.noChat : undefined}
                  onClick={() =>
                    void update({
                      action: 'share',
                      hwnd: selected.hwnd,
                      sessionId: sessionId!,
                      mode: 'view',
                      expectedPid: selected.pid,
                      expectedIdentity: selected.identity
                    })
                  }
                >
                  <Eye size={15} />
                  {c.view}
                </button>
                <button
                  className="automation-primary"
                  disabled={
                    !sessionId ||
                    (share?.hwnd === selected.hwnd &&
                      share.sessionId === sessionId &&
                      share.mode === 'control')
                  }
                  title={!sessionId ? c.noChat : undefined}
                  onClick={() =>
                    void update({
                      action: 'share',
                      hwnd: selected.hwnd,
                      sessionId: sessionId!,
                      mode: 'control',
                      expectedPid: selected.pid,
                      expectedIdentity: selected.identity
                    })
                  }
                >
                  <MousePointer2 size={15} />
                  {c.control}
                </button>
              </div>
              <p className="automation-notice">
                {c.scope} {c.controlHint}
              </p>
            </>
          )}
          <div className="windows-log-heading">
            <h4>{c.log}</h4>
            <span>{state.log.length}</span>
          </div>
          <ul className="windows-log">
            {state.log.slice(0, 15).map((entry, index) => (
              <li key={`${entry.time}-${index}`} className={entry.ok ? '' : 'is-error'}>
                <time>{new Date(entry.time).toLocaleTimeString(locale)}</time>
                <span>
                  {entry.source === 'agent' ? c.agent : c.you} · {entry.title}
                  {entry.error ? ` · ${entry.error}` : ''}
                </span>
              </li>
            ))}
            {!state.log.length && <li>{c.noLog}</li>}
          </ul>
        </section>
      </div>
    </div>
  )
}

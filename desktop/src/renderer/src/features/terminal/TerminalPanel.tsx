import { EmptyState } from '../../components/EmptyState'
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import {
  Check,
  ChevronRight,
  CircleAlert,
  Copy,
  Hand,
  LoaderCircle,
  Plus,
  TerminalSquare,
  Trash2,
  X
} from 'lucide-react'
import type { TerminalApi, TerminalSnapshot } from '../../../../shared/terminal'
import { useI18n } from '../../i18n'
import { terminalError } from './errors'
import { TerminalView, type TerminalHandle, type TerminalTheme } from './TerminalView'
import { useTerminalWorkspace, type TerminalSubscribe } from './useTerminalWorkspace'
import '@xterm/xterm/css/xterm.css'
import '../../styles/terminal.css'

type Props = {
  workspaceId: string
  api: TerminalApi
  subscribe: TerminalSubscribe
  onError: (message: string) => void
  theme?: TerminalTheme
  headerHost?: HTMLElement | null
}

const selectedByWorkspace = new Map<string, string>()

export function TerminalPanel(props: Props) {
  return <TerminalWorkspace key={props.workspaceId} {...props} />
}

function TerminalWorkspace({ workspaceId, api, subscribe, onError, theme, headerHost }: Props) {
  const { t } = useI18n()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)
  const [selected, setSelected] = useState(() => selectedByWorkspace.get(workspaceId) ?? '')
  const handles = useRef(new Map<string, TerminalHandle>())
  const alive = useRef(true)
  const report = (failure: unknown) => {
    if (!alive.current) return
    const message = terminalError(failure, t)
    setError(message)
    onError(message)
  }
  const { terminals, loading } = useTerminalWorkspace(workspaceId, api, subscribe, report)
  const current = terminals.find((terminal) => terminal.id === selected) ?? terminals[0]

  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])
  useEffect(() => {
    if (current) {
      setSelected(current.id)
      selectedByWorkspace.set(workspaceId, current.id)
    }
  }, [current?.id, workspaceId])
  useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), 1800)
    return () => clearTimeout(timer)
  }, [copied])

  const act = async (action: () => Promise<unknown>) => {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      await action()
    } catch (failure) {
      report(failure)
    } finally {
      if (alive.current) setBusy(false)
    }
  }
  const create = () =>
    act(async () => {
      const terminal = (await api({ action: 'create' })) as TerminalSnapshot
      if (alive.current) setSelected(terminal.id)
    })
  const select = (id: string) => {
    setSelected(id)
    setCopied(false)
  }
  const navigateTabs = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    let next = index
    if (event.key === 'ArrowRight') next = (index + 1) % terminals.length
    else if (event.key === 'ArrowLeft') next = (index - 1 + terminals.length) % terminals.length
    else if (event.key === 'Home') next = 0
    else if (event.key === 'End') next = terminals.length - 1
    else return
    event.preventDefault()
    select(terminals[next].id)
    document.getElementById(`terminal-tab-${terminals[next].id}`)?.focus()
  }
  const close = (id: string) => act(() => api({ action: 'close', id }))
  const register = (id: string, handle?: TerminalHandle) => {
    if (handle) handles.current.set(id, handle)
    else handles.current.delete(id)
  }
  const copy = () =>
    act(async () => {
      const handle = current && handles.current.get(current.id)
      if (!handle) return
      await handle.copy()
      if (alive.current) setCopied(true)
    })

  const header = (
    <header className="terminal-header">
      <div className="terminal-heading">
        <TerminalSquare size={15} />
        <span>{t('Терминал')}</span>
        {terminals.length > 0 && <span className="terminal-count">{terminals.length}</span>}
      </div>
      <button
        className="terminal-new"
        onClick={() => void create()}
        disabled={busy || loading}
        title={t('Новый терминал')}
        aria-label={t('Новый терминал')}
      >
        {busy ? <LoaderCircle size={14} className="terminal-spin" /> : <Plus size={14} />}
        <span>{t('Новый')}</span>
      </button>
    </header>
  )

  return (
    <section className="terminal-panel" aria-label={t('Терминал')}>
      {headerHost ? createPortal(header, headerHost) : header}
      {terminals.length > 0 && (
        <div className={`terminal-bar ${current?.owner === 'agent' ? 'agent-owned' : ''}`}>
          <div className="terminal-tabs" role="tablist" aria-label={t('Вкладки терминала')}>
            {terminals.map((terminal, index) => (
              <div
                className={`terminal-tab-wrap ${current?.id === terminal.id ? 'active' : ''}`}
                key={terminal.id}
              >
                <button
                  role="tab"
                  className="terminal-tab"
                  id={`terminal-tab-${terminal.id}`}
                  aria-controls={`terminal-panel-${terminal.id}`}
                  aria-selected={current?.id === terminal.id}
                  tabIndex={current?.id === terminal.id ? 0 : -1}
                  onClick={() => select(terminal.id)}
                  onKeyDown={(event) => navigateTabs(event, index)}
                >
                  <span className={`terminal-status-dot ${terminal.status}`} />
                  <span>{terminal.title.replace(/\.exe$/i, '')}</span>
                  {terminal.owner === 'agent' && <Hand size={11} />}
                </button>
                <button
                  className="terminal-tab-close"
                  aria-label={t('Закрыть терминал {name}', { name: terminal.title })}
                  title={t('Закрыть терминал')}
                  disabled={busy}
                  onClick={() => void close(terminal.id)}
                >
                  <X size={12} />
                </button>
              </div>
            ))}
          </div>
          {current && (
            <div className="terminal-bar-actions">
              <span className="terminal-owner">
                <span className={`terminal-status-dot ${current.status}`} />
                {current.status === 'exited'
                  ? t('Завершён · код {code}', { code: current.exitCode ?? 0 })
                  : current.owner === 'agent'
                    ? t('Управляет агент')
                    : t('Вы управляете')}
              </span>
              <div className="terminal-toolbar-actions">
                <button
                  className="terminal-icon-button"
                  onClick={() => void copy()}
                  title={copied ? t('Скопировано') : t('Копировать вывод')}
                  aria-label={t('Копировать вывод')}
                  disabled={busy}
                >
                  {copied ? <Check size={13} /> : <Copy size={13} />}
                </button>
                <button
                  className="terminal-icon-button"
                  onClick={() => handles.current.get(current.id)?.clear()}
                  title={t('Очистить экран')}
                  aria-label={t('Очистить экран')}
                >
                  <Trash2 size={13} />
                </button>
                {current.status === 'running' && (
                  <button
                    className="terminal-handover"
                    disabled={busy}
                    onClick={() =>
                      void act(() =>
                        api({
                          action: 'owner',
                          id: current.id,
                          owner: current.owner === 'user' ? 'agent' : 'user'
                        })
                      )
                    }
                  >
                    <Hand size={12} />
                    <span>
                      {current.owner === 'agent' ? t('Вернуть управление') : t('Передать агенту')}
                    </span>
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      )}
      {error && (
        <div className="terminal-error" role="alert">
          <CircleAlert size={14} />
          <span>{error}</span>
          <button aria-label={t('Закрыть')} onClick={() => setError('')}>
            <X size={13} />
          </button>
        </div>
      )}
      {loading ? (
        <div className="terminal-empty">
          <LoaderCircle size={22} className="terminal-spin" />
          <p>{t('Загрузка терминалов…')}</p>
        </div>
      ) : !current ? (
        <EmptyState
          compact
          icon={TerminalSquare}
          title={t('Команды в контексте проекта')}
          text={t(
            'Запускайте команды и следите за выводом. Терминал продолжит работать при смене вкладки.'
          )}
          action={{
            label: t('Новый терминал'),
            icon: Plus,
            onClick: () => void create(),
            disabled: busy
          }}
          note={t('Управление остаётся у вас')}
        />
      ) : (
        <>
          {current.owner === 'agent' && current.status === 'running' && (
            <div className="terminal-agent-notice">
              {t('Ввод передан агенту. Вы можете вернуть управление в любой момент.')}
            </div>
          )}
          <div className="terminal-viewport">
            {terminals.map((terminal) => (
              <TerminalView
                key={terminal.id}
                terminal={terminal}
                active={terminal.id === current.id}
                api={api}
                subscribe={subscribe}
                theme={theme}
                onError={report}
                onReady={register}
              />
            ))}
          </div>
          <footer className="terminal-footer">
            <span title={current.cwd}>{current.cwd}</span>
            <span>
              {current.cols} × {current.rows}
            </span>
          </footer>
        </>
      )}
    </section>
  )
}

import { useCallback, useEffect, useMemo, useState } from 'react'
import { Globe, Pencil, Play, Plus, RotateCw, Rocket, Send, Square, Trash2 } from 'lucide-react'
import type { BrowserLog, DesktopEvent } from '../../../../shared/contracts'
import type { LaunchApi, LaunchConfig, LaunchRequest, LaunchResponse } from '../../../../shared/launch'
import type { TerminalApi, TerminalRead, TerminalSnapshot } from '../../../../shared/terminal'
import { TerminalView } from '../terminal/TerminalView'
import { useI18n } from '../../i18n'
import { LaunchEditor } from './LaunchEditor'
import { diagnosticsPrompt } from './diagnostics'
import '../../styles/automations.css'
import '../../styles/launch.css'

type Props = {
  root: string
  api: LaunchApi
  terminal: TerminalApi
  subscribe(listener: (event: DesktopEvent) => void): () => void
  theme?: 'dark' | 'light' | 'system'
  onError(error: unknown): void
  onDiagnostics(text: string): void
}

export function LaunchPanel(props: Props) {
  return <LaunchWorkspace key={props.root} {...props} />
}

function LaunchWorkspace({ root, api, terminal, subscribe, theme, onError, onDiagnostics }: Props) {
  const { t } = useI18n()
  const [snapshot, setSnapshot] = useState<LaunchResponse>({ root, configs: [], processes: [] })
  const [editor, setEditor] = useState<LaunchConfig | 'new'>()
  const [selected, setSelected] = useState<string>()
  const [logs, setLogs] = useState<BrowserLog[]>([])
  const [busy, setBusy] = useState(false)
  const [include, setInclude] = useState({ server: true, browser: true })
  const refresh = useCallback(async () => {
    try {
      setSnapshot(await api({ action: 'list' }))
      setLogs(await window.prime.browserLogs())
    } catch (failure) {
      onError(failure)
    }
  }, [api, onError])
  useEffect(() => {
    void refresh()
    return subscribe((event) => {
      if ((event.type === 'launch' && event.payload?.root === root) || (event.type === 'browser' && event.payload?.action === 'log'))
        void refresh()
    })
  }, [refresh, root, subscribe])
  const request = async (operation: LaunchRequest) => {
    setBusy(true)
    try {
      setSnapshot(await api(operation))
      return true
    } catch (failure) {
      onError(failure)
      return false
    } finally {
      setBusy(false)
    }
  }
  const current = snapshot.configs.find((config) => config.id === selected) ?? snapshot.configs[0]
  const process = snapshot.processes.find((item) => item.configId === current?.id)
  const [terminalSnapshot, setTerminalSnapshot] = useState<TerminalSnapshot>()
  useEffect(() => {
    if (!process) {
      setTerminalSnapshot(undefined)
      return
    }
    void (terminal({ action: 'read', id: process.terminalId }) as Promise<TerminalRead>)
      .then((read) => {
        setTerminalSnapshot(read.terminal)
      })
      .catch(() => setTerminalSnapshot(undefined))
  }, [process?.terminalId, terminal])
  const errors = useMemo(() => logs.filter((log) => log.level === 'error'), [logs])
  const sendDiagnostics = async () => {
    let output = ''
    if (include.server && process) {
      const read = (await terminal({ action: 'read', id: process.terminalId }).catch(() => undefined)) as
        | TerminalRead
        | undefined
      output = read?.output ?? ''
    }
    onDiagnostics(
      diagnosticsPrompt({
        name: current?.name ?? '',
        command: current?.command ?? '',
        status: process?.status ?? 'stopped',
        exitCode: process?.exitCode,
        url: process?.url,
        output: include.server ? output : undefined,
        browser: include.browser ? logs : undefined
      })
    )
  }
  return (
    <div className="automation-panel launch-panel" aria-label={t('Запуск приложений')}>
      <header className="automation-header">
        <div>
          <Rocket size={17} aria-hidden="true" />
          <h2>{t('Запуск приложений')}</h2>
        </div>
        <div className="automation-actions">
          <button className="automation-primary" onClick={() => setEditor('new')} disabled={busy}>
            <Plus size={15} />
            {t('Новая команда')}
          </button>
        </div>
      </header>
      <div className="launch-layout">
        <aside className="launch-list">
          {editor && (
            <LaunchEditor
              config={editor === 'new' ? undefined : editor}
              busy={busy}
              onCancel={() => setEditor(undefined)}
              onSave={async (input) => {
                if (await request({ action: 'save', input })) setEditor(undefined)
              }}
            />
          )}
          {!snapshot.configs.length && !editor && (
            <div className="automation-empty">
              <Rocket size={26} />
              <h3>{t('Команды запуска проекта')}</h3>
              <p>{t('Например: npm run dev. Адрес приложения откроется во встроенном браузере.')}</p>
            </div>
          )}
          {snapshot.configs.map((config) => {
            const item = snapshot.processes.find((entry) => entry.configId === config.id)
            return (
              <button
                key={config.id}
                className={`launch-item ${config.id === current?.id ? 'selected' : ''}`}
                onClick={() => setSelected(config.id)}
              >
                <span className={`launch-dot is-${item?.status ?? 'stopped'}`} aria-hidden="true" />
                <span className="launch-item-text">
                  <strong>{config.name}</strong>
                  <code>{config.command}</code>
                </span>
              </button>
            )
          })}
        </aside>
        {current && (
          <section className="launch-detail">
            <div className="launch-toolbar">
              <strong>{current.name}</strong>
              <span className={`launch-state is-${process?.status ?? 'stopped'}`}>
                {t(
                  process?.status === 'running'
                    ? 'Работает'
                    : process?.status === 'exited'
                      ? 'Завершено с кодом {value0}'
                      : 'Остановлено',
                  { value0: process?.exitCode ?? 0 }
                )}
              </span>
              <span className="team-spacer" />
              {process?.status === 'running' ? (
                <>
                  <button disabled={busy} onClick={() => void request({ action: 'restart', id: current.id })}>
                    <RotateCw size={14} />
                    {t('Перезапустить')}
                  </button>
                  <button disabled={busy} onClick={() => void request({ action: 'stop', id: current.id })}>
                    <Square size={13} />
                    {t('Остановить')}
                  </button>
                </>
              ) : (
                <button
                  className="automation-primary"
                  disabled={busy}
                  onClick={() => void request({ action: 'start', id: current.id })}
                >
                  <Play size={14} />
                  {t('Запустить')}
                </button>
              )}
              <button aria-label={t('Изменить команду')} disabled={busy} onClick={() => setEditor(current)}>
                <Pencil size={13} />
              </button>
              <button
                aria-label={t('Удалить команду')}
                disabled={busy}
                onClick={() => void request({ action: 'remove', id: current.id })}
              >
                <Trash2 size={13} />
              </button>
            </div>
            {process?.url && (
              <div className="launch-url">
                <Globe size={14} />
                <span>{process.url}</span>
                <button onClick={() => void window.prime.browserNavigate(process.url!).catch(onError)}>
                  {t('Открыть предпросмотр')}
                </button>
              </div>
            )}
            <div className="launch-output">
              {terminalSnapshot ? (
                <TerminalView
                  key={terminalSnapshot.id}
                  terminal={terminalSnapshot}
                  active
                  api={terminal}
                  subscribe={subscribe}
                  theme={theme}
                  onError={onError}
                  onReady={() => {}}
                />
              ) : (
                <p className="launch-placeholder">{t('Логи появятся после запуска.')}</p>
              )}
            </div>
            <div className="launch-diagnostics">
              <div className="launch-diagnostics-heading">
                <strong>{t('Ошибки браузера')}</strong>
                <span>{errors.length}</span>
                <span className="team-spacer" />
                <button
                  disabled={!logs.length}
                  onClick={() => void window.prime.browserLogs(true).then(setLogs).catch(onError)}
                >
                  {t('Очистить')}
                </button>
              </div>
              <ul>
                {logs.slice(-6).map((log, index) => (
                  <li key={`${log.time}-${index}`} className={`is-${log.level}`}>
                    {log.message}
                    {log.source ? <small>{` ${log.source}:${log.line ?? 0}`}</small> : null}
                  </li>
                ))}
                {!logs.length && <li className="launch-placeholder">{t('Ошибок нет')}</li>}
              </ul>
              <div className="launch-send">
                <label>
                  <input
                    type="checkbox"
                    checked={include.server}
                    onChange={(event) => setInclude((value) => ({ ...value, server: event.target.checked }))}
                  />
                  {t('Логи сервера')}
                </label>
                <label>
                  <input
                    type="checkbox"
                    checked={include.browser}
                    onChange={(event) => setInclude((value) => ({ ...value, browser: event.target.checked }))}
                  />
                  {t('Ошибки браузера')}
                </label>
                <span className="team-spacer" />
                <button
                  className="automation-primary"
                  disabled={!include.server && !include.browser}
                  onClick={() => void sendDiagnostics()}
                >
                  <Send size={13} />
                  {t('Передать агенту')}
                </button>
              </div>
            </div>
          </section>
        )}
      </div>
    </div>
  )
}

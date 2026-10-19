import { useCallback, useEffect, useState } from 'react'
import { AppWindow, Eye, EyeOff, RefreshCw, ShieldCheck } from 'lucide-react'
import type { WindowInfo, WindowState } from '../../../../shared/computer'
import { useI18n } from '../../i18n'
import '../../styles/automations.css'
import '../../styles/windows.css'

type Props = { workspaceId: string; sessionId?: string; sessionTitle?: string; onError(error: unknown): void }

export function WindowsPanel({ workspaceId, sessionId, sessionTitle, onError }: Props) {
  const { t, locale } = useI18n()
  const [windows, setWindows] = useState<WindowInfo[]>([])
  const [selected, setSelected] = useState<WindowInfo>()
  const [image, setImage] = useState('')
  const [state, setState] = useState<WindowState>({ log: [] })
  const [busy, setBusy] = useState(false)
  const call = useCallback((request: any) => window.prime.windows(workspaceId, request), [workspaceId])
  const refresh = useCallback(async () => {
    try {
      setWindows(await call({ action: 'list' }))
      setState(await call({ action: 'state' }))
    } catch (error) {
      onError(error)
    }
  }, [call, onError])
  useEffect(() => {
    void refresh()
    return window.prime.onEvent((event) => {
      if (event.type === 'windows' && event.workspaceId === workspaceId) setState(event.payload)
    })
  }, [refresh, workspaceId])
  const select = async (item: WindowInfo) => {
    setSelected(item)
    setImage('')
    setBusy(true)
    try {
      setImage((await call({ action: 'snapshot', hwnd: item.hwnd })).image)
    } catch (error) {
      onError(error)
    } finally {
      setBusy(false)
    }
  }
  const share = state.share
  return (
    <div className="automation-panel windows-panel" aria-label={t('Окна')}>
      <header className="automation-header">
        <div>
          <AppWindow size={17} aria-hidden="true" />
          <h2>{t('Окна компьютера')}</h2>
        </div>
        <div className="automation-actions">
          <button aria-label={t('Обновить')} onClick={() => void refresh()}>
            <RefreshCw size={14} />
          </button>
        </div>
      </header>
      {share && (
        <div className="windows-share" role="status">
          <Eye size={15} />
          <span>{t('Агент видит окно «{value0}» в выбранном чате. Только просмотр.', { value0: share.title })}</span>
          <button className="windows-revoke" onClick={() => void call({ action: 'revoke' }).then(setState).catch(onError)}>
            <EyeOff size={14} />
            {t('Отозвать')}
          </button>
        </div>
      )}
      <div className="windows-layout">
        <aside className="windows-list">
          {windows.map((item) => (
            <button
              key={item.hwnd}
              className={`windows-item ${selected?.hwnd === item.hwnd ? 'selected' : ''}`}
              onClick={() => void select(item)}
            >
              <strong>{item.title}</strong>
              <small>
                {item.bounds.width}×{item.bounds.height}
              </small>
            </button>
          ))}
          {!windows.length && <p className="launch-placeholder">{t('Окна не найдены')}</p>}
        </aside>
        <section className="windows-detail">
          {!selected && <p className="launch-placeholder">{t('Выберите окно, чтобы посмотреть снимок')}</p>}
          {selected && (
            <>
              <div className="windows-preview">
                {image ? <img src={image} alt={selected.title} /> : <p className="launch-placeholder">{t(busy ? 'Снимок…' : 'Снимок недоступен')}</p>}
              </div>
              <div className="automation-actions">
                <button className="automation-primary"
                  disabled={!sessionId || share?.hwnd === selected.hwnd}
                  title={sessionId ? undefined : t('Сначала откройте чат')}
                  onClick={() =>
                    void call({ action: 'share', hwnd: selected.hwnd, sessionId: sessionId! }).then(setState).catch(onError)
                  }
                >
                  <ShieldCheck size={14} />
                  {t('Показывать агенту в чате «{value0}»', { value0: sessionTitle || t('Новый чат') })}
                </button>
              </div>
              <p className="automation-notice">
                {t('Агент получит только снимки этого окна во время работы в выбранном чате. Нажатия и ввод текста недоступны.')}
              </p>
            </>
          )}
          <h4>{t('Журнал просмотров')}</h4>
          <ul className="windows-log">
            {state.log.slice(0, 20).map((entry) => (
              <li key={entry.time + entry.title} className={entry.ok ? '' : 'is-error'}>
                <time>{new Date(entry.time).toLocaleTimeString(locale)}</time>
                {t(entry.source === 'agent' ? 'Агент' : 'Вы')} · {entry.title}
                {entry.error ? ` · ${entry.error}` : ''}
              </li>
            ))}
            {!state.log.length && <li className="launch-placeholder">{t('Пока пусто')}</li>}
          </ul>
        </section>
      </div>
    </div>
  )
}

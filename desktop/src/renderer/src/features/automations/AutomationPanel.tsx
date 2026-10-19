import { useEffect, useRef, useState } from 'react'
import { Clock3, Plus, RefreshCw } from 'lucide-react'
import type {
  AutomationApi,
  AutomationRequest,
  AutomationResponse,
  AutomationRule
} from '../../../../shared/automations'
import type { DesktopEvent } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'
import { AutomationEditor } from './AutomationEditor'
import { AutomationHistory } from './AutomationHistory'
import { automationError, isActive, triggerLabel } from './presentation'
import '../../styles/automations.css'

type Props = {
  root: string
  api: AutomationApi
  subscribe(listener: (event: DesktopEvent) => void): () => void
  onError(message: string): void
  onOpenRun?: (run: { root: string; sessionId: string }) => void
}
export function AutomationPanel(props: Props) {
  return <AutomationWorkspace key={props.root} {...props} />
}
function AutomationWorkspace({ root, api, subscribe, onError, onOpenRun }: Props) {
  const { t, locale } = useI18n()
  const [snapshot, setSnapshot] = useState<AutomationResponse>({ root, rules: [], runs: [] })
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [editor, setEditor] = useState<AutomationRule | 'new' | undefined>()
  const mounted = useRef(true)
  const generation = useRef(0)
  const mutationPending = useRef(false)
  const canonicalRoot = useRef(root)
  const rootKey = (value: string) => value.replaceAll('\\', '/').replace(/\/+$/, '').toLowerCase()
  const readOnly = snapshot.error?.includes('automation.corrupt') ?? false
  const report = (failure: unknown) => {
    if (!mounted.current) return
    const message = automationError(failure, t)
    setError(message)
    onError(message)
  }
  const refresh = async () => {
    const current = ++generation.current
    try {
      const result = await api({ action: 'list', root })
      if (mounted.current && current === generation.current) {
        canonicalRoot.current = result.root
        setSnapshot(result)
        setError(result.error ? automationError(result.error, t) : '')
      }
    } catch (failure) {
      report(failure)
    } finally {
      if (mounted.current) setLoading(false)
    }
  }
  useEffect(() => {
    mounted.current = true
    void refresh()
    const unsubscribe = subscribe((event) => {
      if (
        event.type === 'automation' &&
        (!event.payload?.root || rootKey(event.payload.root) === rootKey(canonicalRoot.current))
      )
        void refresh()
    })
    return () => {
      mounted.current = false
      unsubscribe()
    }
  }, [root, api, subscribe])
  const request = async (operation: AutomationRequest) => {
    if (mutationPending.current) return false
    mutationPending.current = true
    setBusy(true)
    setError('')
    try {
      const result = await api(operation)
      if (!mounted.current) return false
      generation.current++
      setSnapshot(result)
      if (result.error) {
        report(result.error)
        return !result.error.includes('automation.corrupt')
      }
      return true
    } catch (failure) {
      report(failure)
      return false
    } finally {
      mutationPending.current = false
      if (mounted.current) setBusy(false)
    }
  }
  return (
    <div className="automation-panel" aria-label={t('Автоматизации')}>
      <header className="automation-header">
        <div>
          <Clock3 size={17} aria-hidden="true" />
          <h2>{t('Автоматизации')}</h2>
          <span className="automation-count">
            {snapshot.rules.filter((rule) => rule.enabled).length}/{snapshot.rules.length}
          </span>
        </div>
        <div className="automation-actions">
          <button
            aria-label={t('Обновить автоматизации')}
            onClick={() => void refresh()}
            disabled={busy}
          >
            <RefreshCw size={15} />
          </button>
          <button
            className="automation-primary"
            onClick={() => setEditor('new')}
            disabled={busy || readOnly}
          >
            <Plus size={15} />
            {t('Новая автоматизация')}
          </button>
        </div>
      </header>
      <div className="automation-content">
        <p className="automation-notice">
          {t('Приложение должно оставаться запущенным. Пропущенные запуски не повторяются.')}
          <br />
          {t('Местное время')} · {Intl.DateTimeFormat().resolvedOptions().timeZone}
        </p>
        {error && (
          <div className="automation-error" role="alert">
            {error}
          </div>
        )}
        {editor && (
          <AutomationEditor
            key={editor === 'new' ? 'new' : editor.id}
            rule={editor === 'new' ? undefined : editor}
            busy={busy}
            onCancel={() => setEditor(undefined)}
            onSave={async (input) => {
              if (await request({ action: 'save', root, input })) setEditor(undefined)
            }}
          />
        )}
        {loading ? (
          <p role="status">{t('Загрузка…')}</p>
        ) : !snapshot.rules.length && !editor && !readOnly ? (
          <div className="automation-empty">
            <Clock3 size={28} />
            <h3>{t('Задачи по расписанию и событиям')}</h3>
            <p>
              {t('Добавьте проверку проекта, ежедневный обзор или задачу при изменении файлов.')}
            </p>
          </div>
        ) : null}
        <div className="automation-rule-list">
          {snapshot.rules.map((rule) => {
            const active = snapshot.runs.some((run) => run.ruleId === rule.id && isActive(run))
            return (
              <article className="automation-rule" key={rule.id}>
                <div className="automation-rule-heading">
                  <h3>{rule.title}</h3>
                  <span
                    className={`automation-status ${rule.enabled ? 'automation-status-enabled' : ''}`}
                  >
                    {t(rule.enabled ? 'Включено' : 'Приостановлено')}
                  </span>
                </div>
                <p className="automation-rule-prompt">{rule.prompt}</p>
                <p className="automation-schedule">{triggerLabel(rule.trigger, t)}</p>
                <p className="automation-muted">
                  {rule.enabled
                    ? rule.nextAt
                      ? t('Следующий запуск: {time}', {
                          time: new Date(rule.nextAt).toLocaleString(locale)
                        })
                      : t('Ожидание события')
                    : t('Расписание приостановлено')}
                  {active ? ` · ${t('Задача выполняется')}` : ''}
                </p>
                <div className="automation-actions">
                  <button
                    className="automation-primary"
                    disabled={busy || active}
                    onClick={() => void request({ action: 'run', root, id: rule.id })}
                  >
                    {t('Запустить')}
                  </button>
                  <button
                    disabled={busy}
                    onClick={() =>
                      void request({
                        action: 'save',
                        root,
                        input: { ...rule, enabled: !rule.enabled }
                      })
                    }
                  >
                    {t(rule.enabled ? 'Приостановить' : 'Возобновить')}
                  </button>
                  <button disabled={busy} onClick={() => setEditor(rule)}>
                    {t('Изменить')}
                  </button>
                  <button
                    disabled={busy || active}
                    title={active ? t('Сначала остановите активный запуск') : undefined}
                    onClick={() => void request({ action: 'remove', root, id: rule.id })}
                  >
                    {t('Удалить')}
                  </button>
                </div>
              </article>
            )
          })}
        </div>
        <AutomationHistory
          runs={snapshot.runs}
          busy={busy}
          onCancel={(runId) => void request({ action: 'cancel', root, runId })}
          onOpenRun={onOpenRun}
        />
      </div>
    </div>
  )
}

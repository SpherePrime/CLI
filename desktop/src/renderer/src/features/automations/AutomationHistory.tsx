import type { AutomationRun } from '../../../../shared/automations'
import { useI18n } from '../../i18n'
import { automationError, isActive, statusLabel } from './presentation'

export function AutomationHistory({
  runs,
  busy,
  onCancel,
  onOpenRun
}: {
  runs: AutomationRun[]
  busy: boolean
  onCancel(runId: string): void
  onOpenRun?: (run: { root: string; sessionId: string }) => void
}) {
  const { t, locale } = useI18n()
  return (
    <section className="automation-history" aria-label={t('История запусков')}>
      <h3>{t('История запусков')}</h3>
      {!runs.length && <p className="automation-muted">{t('Запусков пока нет')}</p>}
      {runs.map((run) => (
        <article className="automation-history-row" key={run.id}>
          <div className="automation-history-main">
            <strong>{run.title}</strong>
            <span className={`automation-status automation-status-${run.status}`}>
              {statusLabel(run, t)}
            </span>
            <time dateTime={new Date(run.createdAt).toISOString()}>
              {new Date(run.createdAt).toLocaleString(locale)}
            </time>
          </div>
          {run.error && <p className="automation-error-text">{automationError(run.error, t)}</p>}
          <div className="automation-actions">
            {isActive(run) && (
              <button disabled={busy} onClick={() => onCancel(run.id)}>
                {t('Остановить запуск')}
              </button>
            )}
            {run.sessionId && onOpenRun && (
              <button onClick={() => onOpenRun({ root: run.root, sessionId: run.sessionId! })}>
                {t('Открыть чат')}
              </button>
            )}
          </div>
        </article>
      ))}
    </section>
  )
}

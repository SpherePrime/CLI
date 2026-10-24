import { Check, Download, HardDrive, LoaderCircle, TriangleAlert } from 'lucide-react'
import type { ReactNode } from 'react'
import type { SttTask } from '../../../../shared/stt'
import { useI18n } from '../../i18n'
import { progressPercent } from './modelProgress'

export function ModelStatusCard({
  name,
  task,
  pending = false,
  installed = false,
  active = false,
  children
}: {
  name: string
  task?: SttTask
  pending?: boolean
  installed?: boolean
  active?: boolean
  children?: ReactNode
}) {
  const { t } = useI18n()
  const currentTask = task?.state === 'running' ? task : undefined
  const running = Boolean(currentTask) || pending
  const failed = !running && task?.state === 'failed'
  const downloading = currentTask?.phase === 'download'
  const state = failed ? 'failed' : running ? 'running' : installed ? 'ready' : 'missing'
  const percent = progressPercent(currentTask?.done, currentTask?.total)
  const phase =
    currentTask?.phase === 'build'
      ? t('Подготовка движка')
      : currentTask?.phase === 'extract'
        ? t('Распаковка модели')
        : downloading
          ? t('Скачивание модели')
          : t('Подготовка модели')
  const status = failed
    ? t('Ошибка')
    : running
      ? phase
      : active
        ? t('Активна')
        : installed
          ? t('Скачана')
          : t('Не скачана')
  const Icon = failed
    ? TriangleAlert
    : running
      ? downloading
        ? Download
        : LoaderCircle
      : installed
        ? Check
        : HardDrive
  return (
    <section className={`model-status is-${state}`} aria-label={name}>
      <div className="model-status-heading">
        <span className="model-status-icon" aria-hidden="true">
          <Icon size={19} className={running && !downloading ? 'spin' : undefined} />
        </span>
        <div className="model-status-title">
          <strong>{name}</strong>
          <span role="status">{status}</span>
        </div>
        {percent !== undefined && (
          <span className="model-status-percent" aria-hidden="true">
            {percent}%
          </span>
        )}
      </div>
      {running && (
        <progress aria-label={t('Подготовка модели {name}', { name })} value={percent} max={100} />
      )}
      {failed && task?.error && (
        <p className="model-status-error" role="alert">
          {task.error}
        </p>
      )}
      {currentTask?.text && (
        <details className="model-status-details">
          <summary>{t('Подробности')}</summary>
          <pre>{currentTask.text}</pre>
        </details>
      )}
      {children}
    </section>
  )
}

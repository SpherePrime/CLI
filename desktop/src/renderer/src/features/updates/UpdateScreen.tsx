import { useEffect, useRef, useState } from 'react'
import { ArrowDown, Check, RefreshCw, TriangleAlert } from 'lucide-react'
import type { UpdateStatus } from '../../../../shared/updates'
import { PrimeLogo } from '../../components/PrimeLogo'
import { useI18n } from '../../i18n'
import { flushDrafts } from '../../host/flushDrafts'

const recheckInterval = 60 * 60 * 1000

export function UpdateScreen({
  ready,
  busy,
  terminals = 0
}: {
  ready: boolean
  busy: boolean
  terminals?: number
}) {
  const { t } = useI18n()
  const [status, setStatus] = useState<UpdateStatus>({ state: 'idle' })
  const [error, setError] = useState('')
  const [forceRestarting, setForceRestarting] = useState(false)
  const checked = useRef(false)
  const latest = useRef(status)
  latest.current = status
  useEffect(() => {
    let alive = true
    let received = false
    const accept = (value: UpdateStatus) => {
      if (alive) setStatus(value)
    }
    const unsubscribe = window.prime.onEvent((event) => {
      if (event.type === 'update') {
        received = true
        accept(event.payload as UpdateStatus)
      }
    })
    void window.prime
      .info()
      .then((info) => {
        if (info.update && !received) accept(info.update as UpdateStatus)
      })
      .catch(() => {})
    return () => {
      alive = false
      unsubscribe()
    }
  }, [])
  useEffect(() => {
    if (!ready || checked.current) return
    checked.current = true
    const check = () =>
      void window.prime
        .update('check')
        .then((value) => setStatus(value as UpdateStatus))
        .catch(() => {})
    check()
    const timer = window.setInterval(() => {
      if (['idle', 'current', 'error', 'unavailable'].includes(latest.current.state)) check()
    }, recheckInterval)
    return () => window.clearInterval(timer)
  }, [ready])
  const downloaded = status.state === 'downloaded'
  const failed = status.state === 'error'
  const installing = status.state === 'installing'
  if (
    !ready ||
    !status.version ||
    !['available', 'downloading', 'downloaded', 'installing', 'error'].includes(status.state)
  )
    return null
  const percent = Math.round(status.percent || 0)
  const megabytes = (bytes?: number) => ((bytes || 0) / 1024 / 1024).toFixed(1)
  return (
    <section
      className={`update-screen ${downloaded || failed ? 'update-compact' : ''}`}
      aria-label={t('Обновление Prime Desktop')}
    >
      <div className="update-card">
        <div className="update-art" aria-hidden="true">
          <div className="update-ring update-ring-one" />
          <div className="update-ring update-ring-two" />
          <div className="update-logo">
            <PrimeLogo />
          </div>
          <span className={`update-phase ${downloaded ? 'complete' : ''}`}>
            {downloaded ? (
              <Check size={18} />
            ) : failed ? (
              <TriangleAlert size={18} />
            ) : (
              <ArrowDown size={18} />
            )}
          </span>
        </div>
        <div className="update-copy" role="status" aria-live="polite">
          <span className="update-eyebrow">PRIME DESKTOP · {status.version}</span>
          <h2>
            {downloaded
              ? t('Необходимо перезапустить приложение')
              : failed
                ? t('Не удалось загрузить обновление')
                : installing
                  ? t('Перезапускаем Prime')
                  : status.state === 'available'
                    ? t('Доступна новая версия')
                    : t('Загружаем обновление')}
          </h2>
          <p>
            {downloaded
              ? t('Новая версия загружена. Перезапустите Prime, чтобы установить её.')
              : failed
                ? t('Проверьте соединение и повторите загрузку.')
                : installing
                  ? t('Устанавливаем новую версию и открываем приложение снова.')
                  : t('Готовим новую версию Prime Desktop.')}
          </p>
        </div>
        {!downloaded && !failed && !installing && (
          <>
            <div
              className="update-progress"
              role="progressbar"
              aria-label={t('Загрузка обновления')}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={percent}
            >
              <span style={{ width: `${Math.max(2, percent)}%` }} />
            </div>
            <div className="update-transfer">
              <span>
                {status.total
                  ? `${megabytes(status.transferred)} / ${megabytes(status.total)} MB`
                  : t('Подготавливаем загрузку…')}
              </span>
              <strong>{percent}%</strong>
            </div>
          </>
        )}
        {(error || (failed && status.message)) && (
          <p role="alert" className="error-text">
            {error || status.message}
          </p>
        )}
        {downloaded && (
          <button
            className="primary update-restart"
            disabled={busy || terminals > 0}
            onClick={() => {
              setError('')
              void flushDrafts().then(() => window.prime.update('install')).catch((error) => setError(error.message))
            }}
          >
            <RefreshCw size={17} />
            {t('Перезапустить')}
          </button>
        )}
        {downloaded && (busy || terminals > 0) && (
          <>
            <p className="muted">
              {terminals > 0
                ? t('Закройте терминалы перед перезапуском.')
                : t('Дождитесь завершения текущей задачи.')}
            </p>
            <button
              className="primary"
              disabled={forceRestarting}
              onClick={() => {
                setError('')
                setForceRestarting(true)
                void flushDrafts()
                  .then(() => window.prime.update('install', true))
                  .catch((error) => setError(error.message))
                  .finally(() => setForceRestarting(false))
              }}
            >
              <RefreshCw size={17} />
              {t('Остановить задачи и перезапустить')}
            </button>
          </>
        )}
        {failed && (
          <button
            className="primary"
            onClick={() => {
              setError('')
              void window.prime
                .update('check')
                .then((value) => setStatus(value as UpdateStatus))
                .catch((error) => setError(error.message))
            }}
          >
            <RefreshCw size={16} />
            {t('Повторить загрузку')}
          </button>
        )}
      </div>
    </section>
  )
}

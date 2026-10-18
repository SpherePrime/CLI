import { useEffect, useRef, useState } from 'react'
import { ArrowDown, Check, RefreshCw, TriangleAlert } from 'lucide-react'
import type { UpdateStatus } from '../../../../shared/updates'
import { PrimeLogo } from '../../components/PrimeLogo'
import { useI18n } from '../../i18n'

export function UpdateScreen({ ready, busy }: { ready: boolean; busy: boolean }) {
  const { t } = useI18n()
  const [status, setStatus] = useState<UpdateStatus>({ state: 'idle' })
  const [error, setError] = useState('')
  const checked = useRef(false)
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
    void window.prime
      .update('check')
      .then((value) => setStatus(value as UpdateStatus))
      .catch(() => {})
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
            disabled={busy}
            onClick={() => {
              setError('')
              void window.prime.update('install').catch((error) => setError(error.message))
            }}
          >
            <RefreshCw size={17} />
            {t('Перезапустить')}
          </button>
        )}
        {downloaded && busy && <p className="muted">{t('Дождитесь завершения текущей задачи.')}</p>}
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

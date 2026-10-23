import { useEffect, useRef, useState } from 'react'
import { AlertTriangle, Check, Copy, X } from 'lucide-react'
import { useI18n } from '../../i18n'
import { dismissToast, subscribeToasts, type Toast } from './notifications'
import './toasts.css'

const lifetime = 9000

function ToastCard({ toast, openProviders }: { toast: Toast; openProviders: () => void }) {
  const { t } = useI18n()
  const [paused, setPaused] = useState(false)
  const [copied, setCopied] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const remaining = useRef(lifetime)
  const started = useRef(Date.now())
  useEffect(() => {
    if (paused) {
      remaining.current -= Date.now() - started.current
      return
    }
    started.current = Date.now()
    const timer = window.setTimeout(() => dismissToast(toast.id), Math.max(remaining.current, 0))
    return () => window.clearTimeout(timer)
  }, [paused, toast.id])
  const long = toast.message.length > 160
  return (
    <div
      className={`toast-card${paused ? ' paused' : ''}`}
      role="alert"
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
    >
      <span className="toast-icon">
        <AlertTriangle size={16} />
      </span>
      <div className="toast-body">
        <strong>{t(toast.title)}</strong>
        <p className={expanded ? 'expanded' : ''}>{toast.message}</p>
        <div className="toast-actions">
          {long && (
            <button onClick={() => setExpanded((value) => !value)}>
              {expanded ? t('Скрыть подробности') : t('Подробнее')}
            </button>
          )}
          {toast.action === 'providers' && (
            <button
              onClick={() => {
                openProviders()
                dismissToast(toast.id)
              }}
            >
              {t('Настройки')}
            </button>
          )}
          <button
            onClick={() =>
              void navigator.clipboard.writeText(toast.message).then(() => setCopied(true))
            }
          >
            {copied ? <Check size={12} /> : <Copy size={12} />}
            {copied ? t('Скопировано') : t('Копировать')}
          </button>
        </div>
      </div>
      <button
        className="toast-close"
        aria-label={t('Скрыть ошибку')}
        onClick={() => dismissToast(toast.id)}
      >
        <X size={14} />
      </button>
      <span
        className="toast-progress"
        style={{ animationDuration: `${lifetime}ms`, animationPlayState: paused ? 'paused' : 'running' }}
      />
    </div>
  )
}

export function ToastStack({ openProviders }: { openProviders: () => void }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  useEffect(() => subscribeToasts(setToasts), [])
  if (!toasts.length) return null
  return (
    <div className="toast-stack" aria-live="assertive">
      {toasts.map((toast) => (
        <ToastCard key={toast.id} toast={toast} openProviders={openProviders} />
      ))}
    </div>
  )
}

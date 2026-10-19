import { useEffect, useState } from 'react'
import { LoaderCircle, X } from 'lucide-react'
import { useI18n } from '../../i18n'
import '../../styles/project-lock.css'

export function ProjectLockBanner() {
  const { t } = useI18n()
  const [path, setPath] = useState('')
  useEffect(
    () =>
      window.prime.onEvent((event) => {
        if (event.type === 'projectLock') setPath(event.payload?.waiting ? String(event.payload.path) : '')
      }),
    []
  )
  if (!path) return null
  return (
    <div className="project-lock-banner" role="status">
      <LoaderCircle size={15} className="spin" />
      <div>
        <strong>{t('Проект открыт в Prime CLI')}</strong>
        <span>
          {t('Закройте CLI в папке {value0} — Desktop откроет проект и его чаты автоматически.', {
            value0: path.split(/[\\/]/).pop() || path
          })}
        </span>
      </div>
      <button
        className="icon small"
        aria-label={t('Отменить открытие')}
        title={t('Отменить открытие')}
        onClick={() => void window.prime.cancelProjectWait()}
      >
        <X size={14} />
      </button>
    </div>
  )
}

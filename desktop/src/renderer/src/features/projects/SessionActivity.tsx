import { LoaderCircle } from 'lucide-react'
import { useI18n } from '../../i18n'
import './sessionActivity.css'

export function SessionActivity() {
  const { t } = useI18n()
  const label = t('Задача выполняется')
  return (
    <LoaderCircle className="session-activity" size={14} role="img" aria-label={label}>
      <title>{label}</title>
    </LoaderCircle>
  )
}

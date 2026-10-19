import { FileCode2 } from 'lucide-react'
import { useI18n } from '../../i18n'

export function MessageAttachment({ data }: { data: Record<string, any> }) {
  const { t } = useI18n()
  const mime = data.mime_type ?? data.MIMEType ?? ''
  const content = data.data ?? data.Data
  const name = data.path ?? data.Path ?? t('Вложенное изображение')
  if (/^image\/(png|jpeg|gif|webp)$/.test(mime) && typeof content === 'string')
    return (
      <div className="attachment-note">
        <img src={`data:${mime};base64,${content}`} alt={name} loading="lazy" />
      </div>
    )
  return (
    <div className="attachment-note">
      <FileCode2 size={14} />
      {t('Вложение ·')} {mime}
    </div>
  )
}

import { FileCode2 } from 'lucide-react'
import { useI18n } from '../../i18n'
import { VideoPlayer } from './video/VideoPlayer'
import { WorkspaceVideo } from './video/WorkspaceVideo'
import { fileNameFromPath, isVideoMime, isVideoPath, videoMimeFromPath } from './video/videoMedia'

export function MessageAttachment({ data }: { data: Record<string, any> }) {
  const { t } = useI18n()
  const mime = String(data.mime_type ?? data.MIMEType ?? '')
  const content = data.data ?? data.Data
  const filePath = data.path ?? data.Path
  const name = filePath ?? t('Вложенное изображение')
  if (/^image\/(png|jpeg|jpg|gif|webp)$/i.test(mime) && typeof content === 'string')
    return (
      <div className="attachment-note">
        <img src={`data:${mime};base64,${content}`} alt={name} loading="lazy" />
      </div>
    )
  if (isVideoMime(mime) || (!mime && typeof filePath === 'string' && isVideoPath(filePath))) {
    const resolvedMime = mime || (typeof filePath === 'string' ? videoMimeFromPath(filePath) : undefined) || 'video/mp4'
    const videoTitle =
      typeof filePath === 'string' && filePath
        ? fileNameFromPath(filePath) || filePath
        : t('Видео')
    if (typeof content === 'string' && content) {
      return (
        <VideoPlayer
          src={`data:${resolvedMime};base64,${content}`}
          title={videoTitle}
          mime={resolvedMime}
          variant="chat"
        />
      )
    }
    if (typeof filePath === 'string' && filePath) {
      return <WorkspaceVideo src={filePath} title={videoTitle} mime={resolvedMime} variant="chat" />
    }
  }
  return (
    <div className="attachment-note">
      <FileCode2 size={14} />
      {t('Вложение ·')} {mime}
    </div>
  )
}

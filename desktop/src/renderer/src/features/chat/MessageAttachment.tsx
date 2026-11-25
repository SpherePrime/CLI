import {
  ExternalLink,
  FileCode2,
  FileSpreadsheet,
  FileText,
  FolderOpen,
  Presentation
} from 'lucide-react'
import { useI18n } from '../../i18n'
import { useWorkspaceMediaId } from './WorkspaceMediaContext'
import { VideoPlayer } from './video/VideoPlayer'
import { WorkspaceVideo } from './video/WorkspaceVideo'
import { fileNameFromPath, isVideoMime, isVideoPath, videoMimeFromPath } from './video/videoMedia'
import '../../styles/attachments.css'

function extensionFromNameOrMime(fileName: string, mime: string): string {
  const match = /\.([a-zA-Z0-9]{1,10})$/.exec(fileName.trim())
  if (match) return match[1].toLowerCase()
  const lowerMime = mime.toLowerCase().split(';')[0].trim()
  if (lowerMime === 'application/pdf') return 'pdf'
  if (lowerMime === 'application/vnd.openxmlformats-officedocument.wordprocessingml.document')
    return 'docx'
  if (lowerMime === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet')
    return 'xlsx'
  if (lowerMime === 'application/vnd.openxmlformats-officedocument.presentationml.presentation')
    return 'pptx'
  if (lowerMime === 'text/markdown') return 'md'
  if (lowerMime === 'text/csv') return 'csv'
  if (lowerMime === 'text/plain') return 'txt'
  return ''
}

function DocumentIcon({ ext }: { ext: string }) {
  if (ext === 'pdf' || ext === 'docx' || ext === 'doc') return <FileText size={16} />
  if (ext === 'xlsx' || ext === 'xls' || ext === 'csv') return <FileSpreadsheet size={16} />
  if (ext === 'pptx' || ext === 'ppt') return <Presentation size={16} />
  return <FileCode2 size={16} />
}

export function MessageAttachment({ data }: { data: Record<string, any> }) {
  const { t } = useI18n()
  const workspaceId = useWorkspaceMediaId()
  const mime = String(data.mime_type ?? data.MIMEType ?? '')
  const content = data.data ?? data.Data
  const filePath = typeof (data.path ?? data.Path) === 'string' ? String(data.path ?? data.Path) : ''
  const name = filePath || t('Вложенное изображение')
  if (/^image\/(png|jpeg|jpg|gif|webp)$/i.test(mime) && typeof content === 'string')
    return (
      <div className="attachment-note">
        <img src={`data:${mime};base64,${content}`} alt={name} loading="lazy" />
      </div>
    )
  if (isVideoMime(mime) || (!mime && filePath && isVideoPath(filePath))) {
    const resolvedMime = mime || (filePath ? videoMimeFromPath(filePath) : undefined) || 'video/mp4'
    const videoTitle = filePath ? fileNameFromPath(filePath) || filePath : t('Видео')
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
    if (filePath) {
      return <WorkspaceVideo src={filePath} title={videoTitle} mime={resolvedMime} variant="chat" />
    }
  }
  const displayName =
    (filePath ? fileNameFromPath(filePath) || filePath : '') ||
    String(data.name ?? data.file_name ?? '') ||
    t('Вложение')
  const ext = extensionFromNameOrMime(displayName, mime)
  const badge = ext ? ext.toUpperCase() : ''
  const hasPath = filePath.trim().length > 0

  return (
    <div className={`attachment-document-card${ext ? ` is-${ext}` : ''}`}>
      <span className="attachment-document-icon" aria-hidden="true">
        <DocumentIcon ext={ext} />
      </span>
      <span className="attachment-document-meta">
        <span className="attachment-document-name" title={filePath || displayName}>
          {displayName}
        </span>
        {badge && <span className="attachment-document-badge">{badge}</span>}
      </span>
      {hasPath && (
        <span className="attachment-document-actions">
          <button
            type="button"
            className="attachment-document-action"
            title={t('Открыть')}
            aria-label={t('Открыть')}
            onClick={() => void window.prime?.openPath?.(workspaceId || '', filePath)}
          >
            <ExternalLink size={13} />
          </button>
          <button
            type="button"
            className="attachment-document-action"
            title={t('Показать в папке')}
            aria-label={t('Показать в папке')}
            onClick={() => void window.prime?.reveal?.(workspaceId || '', filePath)}
          >
            <FolderOpen size={13} />
          </button>
        </span>
      )}
    </div>
  )
}

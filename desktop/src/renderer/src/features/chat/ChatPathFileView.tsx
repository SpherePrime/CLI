import {
  ExternalLink,
  File,
  FileSpreadsheet,
  FileText,
  FolderOpen,
  Presentation
} from 'lucide-react'
import type { TranslationValues } from '../../../../shared/localization'
import { getPathExtension } from './chatPaths'

export type ChatFileTone = 'pdf' | 'doc' | 'sheet' | 'slides' | 'neutral'

type Translator = (source: string, values?: TranslationValues) => string

export function resolveFileTone(extension: string): ChatFileTone {
  switch (extension.toLowerCase()) {
    case 'pdf':
      return 'pdf'
    case 'docx':
    case 'doc':
      return 'doc'
    case 'xlsx':
    case 'xls':
    case 'csv':
      return 'sheet'
    case 'pptx':
    case 'ppt':
      return 'slides'
    default:
      return 'neutral'
  }
}

export function formatChatFileSize(bytes: number | undefined, t: Translator): string {
  if (typeof bytes !== 'number' || !Number.isFinite(bytes) || bytes < 0) return ''
  if (bytes < 1024) {
    return t('{value0} Б', { value0: Math.round(bytes) })
  }
  if (bytes < 1024 ** 2) {
    const kb = bytes / 1024
    const formatted = kb >= 10 || Number.isInteger(kb) ? Math.round(kb) : Number(kb.toFixed(1))
    return t('{value0} КБ', { value0: formatted })
  }
  if (bytes < 1024 ** 3) {
    const mb = bytes / 1024 ** 2
    const formatted = mb >= 100 ? Math.round(mb) : Number(mb.toFixed(1))
    return t('{value0} МБ', { value0: formatted })
  }
  const gb = bytes / 1024 ** 3
  const formatted = gb >= 100 ? Math.round(gb) : Number(gb.toFixed(1))
  return t('{value0} ГБ', { value0: formatted })
}

export function openChatPathTarget(workspaceId: string, targetPath: string) {
  const api = typeof window !== 'undefined' ? window.prime : undefined
  if (api?.openPath) {
    void Promise.resolve(api.openPath(workspaceId || '', targetPath)).catch(() => {})
    return
  }
  if (api?.reveal) {
    void Promise.resolve(api.reveal(workspaceId || '', targetPath)).catch(() => {})
  }
}

export function revealChatPathTarget(workspaceId: string, targetPath: string) {
  const api = typeof window !== 'undefined' ? window.prime : undefined
  if (api?.reveal) {
    void Promise.resolve(api.reveal(workspaceId || '', targetPath)).catch(() => {})
  }
}

function FileKindIcon({ tone, size }: { tone: ChatFileTone; size: number }) {
  if (tone === 'pdf' || tone === 'doc') return <FileText size={size} aria-hidden="true" />
  if (tone === 'sheet') return <FileSpreadsheet size={size} aria-hidden="true" />
  if (tone === 'slides') return <Presentation size={size} aria-hidden="true" />
  return <File size={size} aria-hidden="true" />
}

export interface ChatPathFileViewProps {
  variant: 'block' | 'inline'
  workspaceId: string
  targetPath: string
  displayName: string
  extension?: string
  size?: number
  t: Translator
}

export function ChatPathFileView({
  variant,
  workspaceId,
  targetPath,
  displayName,
  extension,
  size,
  t
}: ChatPathFileViewProps) {
  const normalizedExtension = (extension || getPathExtension(targetPath) || '').toLowerCase()
  const tone = resolveFileTone(normalizedExtension)
  const badge = normalizedExtension ? normalizedExtension.toUpperCase() : t('Файл')
  const formattedSize = formatChatFileSize(size, t)
  const openLabel = t('Открыть {value0}', { value0: displayName })
  const revealLabel = t('Показать в папке')

  const handleOpen = () => openChatPathTarget(workspaceId, targetPath)
  const handleReveal = () => revealChatPathTarget(workspaceId, targetPath)

  if (variant === 'inline') {
    return (
      <span className="chat-path-inline-group">
        <button
          type="button"
          className={`chat-path-chip is-file is-${tone}`}
          title={targetPath}
          aria-label={openLabel}
          onClick={handleOpen}
        >
          <span className="chat-path-chip-icon" aria-hidden="true">
            <FileKindIcon tone={tone} size={13} />
          </span>
          <span className="chat-path-chip-name">{displayName}</span>
        </button>
        <button
          type="button"
          className="chat-path-reveal-icon"
          aria-label={revealLabel}
          title={revealLabel}
          onClick={handleReveal}
        >
          <FolderOpen size={12} aria-hidden="true" />
        </button>
      </span>
    )
  }

  return (
    <div className="chat-path-file-card-wrap">
      <div className={`chat-path-file-card is-${tone}`} title={targetPath}>
        <div className="chat-path-file-main">
          <div className="chat-path-file-icon" aria-hidden="true">
            <FileKindIcon tone={tone} size={17} />
          </div>
          <div className="chat-path-file-meta">
            <strong className="chat-path-file-name">{displayName}</strong>
            <span className="chat-path-file-details">
              <span className="chat-path-file-badge">{badge}</span>
              {formattedSize && (
                <>
                  <span className="chat-path-file-sep" aria-hidden="true">
                    ·
                  </span>
                  <span className="chat-path-file-size">{formattedSize}</span>
                </>
              )}
            </span>
          </div>
        </div>
        <div className="chat-path-file-actions">
          <button
            type="button"
            className="chat-path-open-btn"
            aria-label={openLabel}
            title={t('Открыть')}
            onClick={handleOpen}
          >
            <ExternalLink size={13} aria-hidden="true" />
            <span>{t('Открыть')}</span>
          </button>
          <button
            type="button"
            className="chat-path-file-reveal-btn"
            aria-label={revealLabel}
            title={revealLabel}
            onClick={handleReveal}
          >
            <FolderOpen size={14} aria-hidden="true" />
          </button>
        </div>
      </div>
    </div>
  )
}

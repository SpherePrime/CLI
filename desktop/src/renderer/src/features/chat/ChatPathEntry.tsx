import { useEffect, useState, type ReactNode } from 'react'
import { ExternalLink, Film, FolderOpen, Image as ImageIcon } from 'lucide-react'
import type { InspectedPath } from '../../../../shared/contracts'
import { ImageLightbox } from '../../components/ImageLightbox'
import { useI18n } from '../../i18n'
import { useWorkspaceMediaId } from './WorkspaceMediaContext'
import { WorkspaceVideo } from './video/WorkspaceVideo'
import { fileNameFromPath, isDirectMediaUrl } from './video/videoMedia'
import { isImagePath } from './chatPaths'
import './chatPaths.css'

type CachedInspection = {
  value: InspectedPath
  expiresAt: number
}

const inspectionCache = new Map<string, CachedInspection>()
const POSITIVE_TTL_MS = 30_000
const NEGATIVE_TTL_MS = 3_000

function readCachedInspection(key: string): InspectedPath | null {
  const entry = inspectionCache.get(key)
  if (!entry) return null
  if (Date.now() > entry.expiresAt) {
    inspectionCache.delete(key)
    return null
  }
  return entry.value
}

function writeCachedInspection(key: string, value: InspectedPath) {
  if (inspectionCache.size > 400) inspectionCache.clear()
  const ttl = value.exists ? POSITIVE_TTL_MS : NEGATIVE_TTL_MS
  inspectionCache.set(key, { value, expiresAt: Date.now() + ttl })
}

export function clearChatPathInspectionCache() {
  inspectionCache.clear()
}

export function seedChatPathInspectionCache(workspaceId: string, path: string, value: InspectedPath) {
  writeCachedInspection(`${workspaceId}::${path.trim()}`, value)
}

export interface ChatPathEntryProps {
  path: string
  variant: 'block' | 'inline'
  label?: string
  fallback?: ReactNode
  wrapFallbackInParagraph?: boolean
}

export function ChatPathEntry({
  path,
  variant,
  label,
  fallback,
  wrapFallbackInParagraph = false
}: ChatPathEntryProps) {
  const { t } = useI18n()
  const workspaceId = useWorkspaceMediaId()
  const trimmed = path.trim()
  const cacheKey = `${workspaceId}::${trimmed}`

  const [inspected, setInspected] = useState<InspectedPath | null>(() =>
    readCachedInspection(cacheKey)
  )
  const [mediaUrl, setMediaUrl] = useState(() => (isDirectMediaUrl(trimmed) ? trimmed : ''))
  const [mediaFailed, setMediaFailed] = useState(false)
  const [lightboxOpen, setLightboxOpen] = useState(false)

  useEffect(() => {
    const cached = readCachedInspection(cacheKey)
    if (cached) {
      setInspected(cached)
      return
    }
    const inspectApi = typeof window !== 'undefined' ? window.prime?.inspectPath : undefined
    if (!inspectApi) {
      setInspected({ exists: false, kind: 'missing' })
      return
    }
    let cancelled = false
    void inspectApi(workspaceId || '', trimmed)
      .then((result) => {
        if (cancelled) return
        const normalized: InspectedPath = result || { exists: false, kind: 'missing' }
        writeCachedInspection(cacheKey, normalized)
        setInspected(normalized)
      })
      .catch(() => {
        if (cancelled) return
        const fallbackResult: InspectedPath = { exists: false, kind: 'missing' }
        writeCachedInspection(cacheKey, fallbackResult)
        setInspected(fallbackResult)
      })
    return () => {
      cancelled = true
    }
  }, [cacheKey, trimmed, workspaceId])

  useEffect(() => {
    if (!inspected?.exists || inspected.kind !== 'image' || !inspected.previewable) {
      setMediaUrl(isDirectMediaUrl(trimmed) ? trimmed : '')
      return
    }
    const targetPath = inspected.path || trimmed
    if (isDirectMediaUrl(targetPath)) {
      setMediaUrl(targetPath)
      setMediaFailed(false)
      return
    }
    const mediaApi = typeof window !== 'undefined' ? window.prime?.mediaUrl : undefined
    if (!mediaApi) {
      setMediaFailed(true)
      return
    }
    let cancelled = false
    setMediaFailed(false)
    void mediaApi(workspaceId || '', targetPath)
      .then((result) => {
        if (cancelled) return
        if (result?.url) {
          setMediaUrl(result.url)
        } else {
          setMediaFailed(true)
        }
      })
      .catch(() => {
        if (cancelled) return
        setMediaFailed(true)
      })
    return () => {
      cancelled = true
    }
  }, [inspected, trimmed, workspaceId])

  const renderFallback = () => {
    const content = fallback ?? trimmed
    return wrapFallbackInParagraph ? <p>{content}</p> : <>{content}</>
  }

  if (!inspected || !inspected.exists) {
    if (isDirectMediaUrl(trimmed) && isImagePath(trimmed)) {
      return <img src={trimmed} alt={label || fileNameFromPath(trimmed)} loading="lazy" />
    }
    return renderFallback()
  }

  if (inspected.kind === 'file') {
    return renderFallback()
  }

  const targetPath = inspected.path || trimmed
  const displayName = label || inspected.name || fileNameFromPath(targetPath) || targetPath
  const openInExplorer = () => {
    void window.prime?.reveal?.(workspaceId || '', targetPath)
  }

  if (inspected.kind === 'dir') {
    if (variant === 'inline') {
      return (
        <button
          type="button"
          className="chat-path-chip is-dir"
          title={targetPath}
          onClick={openInExplorer}
        >
          <FolderOpen size={13} aria-hidden="true" />
          <span className="chat-path-chip-name">{displayName}</span>
          <ExternalLink size={11} className="chat-path-chip-action" aria-hidden="true" />
        </button>
      )
    }

    return (
      <div className="chat-path-folder-card" title={targetPath}>
        <div className="chat-path-folder-icon" aria-hidden="true">
          <FolderOpen size={16} />
        </div>
        <div className="chat-path-folder-meta">
          <strong>{displayName}</strong>
          <span>{targetPath}</span>
        </div>
        <button type="button" className="chat-path-open-btn" onClick={openInExplorer}>
          <FolderOpen size={13} aria-hidden="true" />
          <span>{t('Открыть')}</span>
        </button>
      </div>
    )
  }

  if (inspected.kind === 'video') {
    if (variant === 'inline') {
      return (
        <span className="chat-path-inline-group">
          {inspected.previewable ? (
            <WorkspaceVideo
              src={targetPath}
              title={displayName}
              mime={inspected.mime}
              inline
            >
              {displayName}
            </WorkspaceVideo>
          ) : (
            <button
              type="button"
              className="chat-path-chip is-video"
              title={targetPath}
              onClick={openInExplorer}
            >
              <Film size={13} aria-hidden="true" />
              <span className="chat-path-chip-name">{displayName}</span>
            </button>
          )}
          <button
            type="button"
            className="chat-path-reveal-icon"
            aria-label={t('Открыть в проводнике')}
            title={t('Открыть в проводнике')}
            onClick={openInExplorer}
          >
            <FolderOpen size={12} aria-hidden="true" />
          </button>
        </span>
      )
    }

    return (
      <div className="chat-path-media-card is-video">
        {inspected.previewable ? (
          <WorkspaceVideo
            src={targetPath}
            title={displayName}
            mime={inspected.mime}
            variant="chat"
          />
        ) : (
          <div className="chat-path-media-fallback">
            <Film size={18} aria-hidden="true" />
            <span>{displayName}</span>
          </div>
        )}
        <div className="chat-path-card-bar">
          <div className="chat-path-card-title" title={targetPath}>
            <Film size={13} aria-hidden="true" />
            <span>{displayName}</span>
          </div>
          <button type="button" className="chat-path-open-btn" onClick={openInExplorer}>
            <FolderOpen size={13} aria-hidden="true" />
            <span>{t('Открыть')}</span>
          </button>
        </div>
      </div>
    )
  }

  if (inspected.kind === 'image') {
    const canShowImage = Boolean(inspected.previewable && mediaUrl && !mediaFailed)
    if (variant === 'inline') {
      return (
        <span className="chat-path-inline-group">
          {canShowImage ? (
            <button
              type="button"
              className="chat-path-chip is-image"
              title={targetPath}
              onClick={() => setLightboxOpen(true)}
            >
              <img src={mediaUrl} alt={displayName} loading="lazy" />
              <span className="chat-path-chip-name">{displayName}</span>
            </button>
          ) : (
            <button
              type="button"
              className="chat-path-chip is-image"
              title={targetPath}
              onClick={openInExplorer}
            >
              <ImageIcon size={13} aria-hidden="true" />
              <span className="chat-path-chip-name">{displayName}</span>
            </button>
          )}
          <button
            type="button"
            className="chat-path-reveal-icon"
            aria-label={t('Открыть в проводнике')}
            title={t('Открыть в проводнике')}
            onClick={openInExplorer}
          >
            <FolderOpen size={12} aria-hidden="true" />
          </button>
          {lightboxOpen && canShowImage && (
            <ImageLightbox
              src={mediaUrl}
              alt={displayName}
              caption={targetPath}
              close={() => setLightboxOpen(false)}
            />
          )}
        </span>
      )
    }

    return (
      <div className="chat-path-media-card is-image">
        {canShowImage ? (
          <button
            type="button"
            className="chat-path-photo-preview"
            aria-label={displayName}
            title={t('Увеличить')}
            onClick={() => setLightboxOpen(true)}
          >
            <img
              src={mediaUrl}
              alt={displayName}
              loading="lazy"
              decoding="async"
              onError={() => setMediaFailed(true)}
            />
          </button>
        ) : (
          <div className="chat-path-media-fallback">
            <ImageIcon size={18} aria-hidden="true" />
            <span>{displayName}</span>
          </div>
        )}
        <div className="chat-path-card-bar">
          <div className="chat-path-card-title" title={targetPath}>
            <ImageIcon size={13} aria-hidden="true" />
            <span>{displayName}</span>
          </div>
          <button type="button" className="chat-path-open-btn" onClick={openInExplorer}>
            <FolderOpen size={13} aria-hidden="true" />
            <span>{t('Открыть')}</span>
          </button>
        </div>
        {lightboxOpen && canShowImage && (
          <ImageLightbox
            src={mediaUrl}
            alt={displayName}
            caption={targetPath}
            close={() => setLightboxOpen(false)}
          />
        )}
      </div>
    )
  }

  return <>{fallback ?? trimmed}</>
}

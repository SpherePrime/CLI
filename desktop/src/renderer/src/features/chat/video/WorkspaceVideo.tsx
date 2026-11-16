import { useEffect, useState, type ReactNode } from 'react'
import { Film } from 'lucide-react'
import { useI18n } from '../../../i18n'
import { useWorkspaceMediaId } from '../WorkspaceMediaContext'
import { VideoLightbox } from './VideoLightbox'
import { VideoPlayer, type VideoPlayerVariant } from './VideoPlayer'
import { fileNameFromPath, isDirectMediaUrl, videoMimeFromPath } from './videoMedia'

export interface WorkspaceVideoProps {
  src: string
  title?: string
  mime?: string
  variant?: VideoPlayerVariant
  inline?: boolean
  children?: ReactNode
}

export function WorkspaceVideo({
  src,
  title,
  mime,
  variant = 'chat',
  inline = false,
  children
}: WorkspaceVideoProps) {
  const { t } = useI18n()
  const workspaceId = useWorkspaceMediaId()
  const direct = isDirectMediaUrl(src)
  const [resolvedUrl, setResolvedUrl] = useState(direct ? src : '')
  const [resolvedMime, setResolvedMime] = useState(mime || videoMimeFromPath(src))
  const [resolvedName, setResolvedName] = useState(title || fileNameFromPath(src) || t('Видео'))
  const [loading, setLoading] = useState(!direct && !inline)
  const [error, setError] = useState(false)
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const [retryTick, setRetryTick] = useState(0)

  const shouldResolve = !direct && (!inline || lightboxOpen)

  useEffect(() => {
    if (direct) {
      setResolvedUrl(src)
      setResolvedMime(mime || videoMimeFromPath(src))
      setResolvedName(title || fileNameFromPath(src) || t('Видео'))
      setLoading(false)
      setError(false)
      return
    }

    if (!shouldResolve) return

    const mediaApi = typeof window !== 'undefined' ? window.prime?.mediaUrl : undefined
    if (!mediaApi) {
      setResolvedUrl('')
      setResolvedMime(mime || videoMimeFromPath(src))
      setResolvedName(title || fileNameFromPath(src) || t('Видео'))
      setLoading(false)
      setError(true)
      return
    }

    let cancelled = false
    setLoading(true)
    setError(false)

    void mediaApi(workspaceId || '', src)
      .then((result) => {
        if (cancelled) return
        if (!result?.url) {
          setResolvedUrl('')
          setError(true)
          setLoading(false)
          return
        }
        setResolvedUrl(result.url)
        setResolvedMime(mime || result.mime || videoMimeFromPath(src))
        setResolvedName(title || result.name || fileNameFromPath(src) || t('Видео'))
        setLoading(false)
        setError(false)
      })
      .catch(() => {
        if (cancelled) return
        setResolvedUrl('')
        setLoading(false)
        setError(true)
      })

    return () => {
      cancelled = true
    }
  }, [src, workspaceId, direct, shouldResolve, retryTick, mime, title, t])

  const retry = () => setRetryTick((tick) => tick + 1)
  const displayTitle = title || resolvedName || fileNameFromPath(src) || t('Видео')
  const isResolving = loading || (!direct && shouldResolve && !resolvedUrl && !error)

  if (inline) {
    return (
      <>
        <button
          type="button"
          className="video-inline-chip"
          title={displayTitle}
          onClick={() => setLightboxOpen(true)}
        >
          <Film size={13} aria-hidden="true" />
          <span>{children || displayTitle}</span>
        </button>
        {lightboxOpen && (
          <VideoLightbox
            src={resolvedUrl}
            title={displayTitle}
            mime={resolvedMime}
            loading={isResolving}
            error={error}
            onRetry={retry}
            close={() => setLightboxOpen(false)}
          />
        )}
      </>
    )
  }

  return (
    <>
      <VideoPlayer
        src={resolvedUrl}
        title={displayTitle}
        mime={resolvedMime}
        variant={variant}
        loading={isResolving}
        error={error}
        onRetry={retry}
        onExpand={variant !== 'lightbox' ? () => setLightboxOpen(true) : undefined}
      />
      {variant !== 'lightbox' && lightboxOpen && (
        <VideoLightbox
          src={resolvedUrl}
          title={displayTitle}
          mime={resolvedMime}
          loading={loading}
          error={error}
          onRetry={retry}
          close={() => setLightboxOpen(false)}
        />
      )}
    </>
  )
}

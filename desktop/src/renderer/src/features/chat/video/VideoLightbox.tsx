import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { useI18n } from '../../../i18n'
import { VideoPlayer } from './VideoPlayer'

export interface VideoLightboxProps {
  src: string
  title?: string
  mime?: string
  loading?: boolean
  error?: boolean
  onRetry?: () => void
  close: () => void
}

export function VideoLightbox({
  src,
  title,
  mime,
  loading,
  error,
  onRetry,
  close
}: VideoLightboxProps) {
  const { t } = useI18n()
  const ref = useRef<HTMLDialogElement>(null)

  useEffect(() => {
    ref.current?.showModal?.()
    return () => ref.current?.close?.()
  }, [])

  return (
    <dialog
      ref={ref}
      className="video-lightbox"
      aria-label={title || t('Видео')}
      onCancel={(event) => {
        event.preventDefault()
        close()
      }}
      onClick={(event) => {
        if (event.target === ref.current) close()
      }}
    >
      <div className="video-lightbox-frame">
        <button
          type="button"
          className="icon video-lightbox-close"
          aria-label={t('Закрыть видео')}
          title={t('Закрыть видео')}
          onClick={close}
        >
          <X size={18} aria-hidden="true" />
        </button>
        <VideoPlayer
          src={src}
          title={title}
          mime={mime}
          variant="lightbox"
          autoPlay
          loading={loading}
          error={error}
          onRetry={onRetry}
        />
      </div>
    </dialog>
  )
}

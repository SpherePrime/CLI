import { useI18n } from '../i18n'
import { useEffect, useRef } from 'react'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'

type LightboxProps = {
  src: string
  alt?: string
  caption?: string
  close: () => void
  previous?: () => void
  next?: () => void
}

export function ImageLightbox({ src, alt, caption, close, previous, next }: LightboxProps) {
  const { t } = useI18n()
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    ref.current?.showModal()
    return () => ref.current?.close()
  }, [])
  return (
    <dialog
      ref={ref}
      className={`image-lightbox ${previous || next ? 'gallery' : ''}`}
      aria-label={t('Изображение')}
      onCancel={(event) => {
        event.preventDefault()
        close()
      }}
      onClick={(event) => {
        if (event.target === ref.current) close()
      }}
      onKeyDown={(event) => {
        if (event.key === 'ArrowLeft' && previous) previous()
        if (event.key === 'ArrowRight' && next) next()
      }}
    >
      <button className="icon" aria-label={t('Закрыть')} onClick={close}>
        <X size={18} />
      </button>
      {previous && (
        <button className="icon lightbox-nav previous" aria-label={t('Предыдущее')} onClick={previous}>
          <ChevronLeft size={20} />
        </button>
      )}
      {next && (
        <button className="icon lightbox-nav next" aria-label={t('Следующее')} onClick={next}>
          <ChevronRight size={20} />
        </button>
      )}
      <img src={src} alt={alt || ''} />
      {caption && <div className="lightbox-caption">{caption}</div>}
    </dialog>
  )
}

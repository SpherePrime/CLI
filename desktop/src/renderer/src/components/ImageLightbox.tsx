import { useI18n } from '../i18n'
import { useEffect } from 'react'
import { X } from 'lucide-react'

export function ImageLightbox({ src, alt, close }: { src: string; alt?: string; close: () => void }) {
  const { t } = useI18n()
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') close()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [close])
  return (
    <div
      className="image-lightbox"
      role="dialog"
      aria-label={t('Изображение')}
      onClick={close}
    >
      <button
        className="icon"
        aria-label={t('Закрыть')}
        onClick={(event) => {
          event.stopPropagation()
          close()
        }}
      >
        <X size={18} />
      </button>
      <img src={src} alt={alt || ''} onClick={(event) => event.stopPropagation()} />
    </div>
  )
}

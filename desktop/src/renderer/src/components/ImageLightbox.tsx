import { useI18n } from '../i18n'
import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'

export function ImageLightbox({ src, alt, close }: { src: string; alt?: string; close: () => void }) {
  const { t } = useI18n()
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    ref.current?.showModal()
    return () => ref.current?.close()
  }, [])
  return (
    <dialog
      ref={ref}
      className="image-lightbox"
      aria-label={t('Изображение')}
      onCancel={(event) => {
        event.preventDefault()
        close()
      }}
      onClick={(event) => {
        if (event.target === ref.current) close()
      }}
    >
      <button className="icon" aria-label={t('Закрыть')} onClick={close}>
        <X size={18} />
      </button>
      <img src={src} alt={alt || ''} />
    </dialog>
  )
}

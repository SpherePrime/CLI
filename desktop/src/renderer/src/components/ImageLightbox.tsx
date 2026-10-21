import { useI18n } from '../i18n'
import { Modal } from './Modal'

export function ImageLightbox({ src, alt, close }: { src: string; alt?: string; close: () => void }) {
  const { t } = useI18n()
  return (
    <Modal title={t('Изображение')} close={close} wide className="image-lightbox">
      <img src={src} alt={alt || ''} />
    </Modal>
  )
}

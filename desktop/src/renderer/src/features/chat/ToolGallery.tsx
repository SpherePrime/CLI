import { useState } from 'react'
import { ImageLightbox } from '../../components/ImageLightbox'
import { useI18n } from '../../i18n'

export type ToolImage = { data: string; mime_type: string; path?: string }

// collectToolImages gathers every image attached to a tool call or result:
// the single legacy Data slot first, then the additional Images array.
export function collectToolImages(result: any): ToolImage[] {
  if (!result || typeof result !== 'object') return []
  const images: ToolImage[] = []
  if (
    typeof result.mime_type === 'string' &&
    result.mime_type.startsWith('image/') &&
    typeof result.data === 'string' &&
    result.data
  )
    images.push({ data: result.data, mime_type: result.mime_type })
  const extra = Array.isArray(result.images) ? result.images : []
  for (const [index, image] of extra.entries()) {
    if (
      typeof image?.mime_type === 'string' &&
      image.mime_type.startsWith('image/') &&
      typeof image?.data === 'string' &&
      image.data &&
      !(index === 0 && images[0]?.data === image.data)
    )
      images.push({ data: image.data, mime_type: image.mime_type, path: image.path })
  }
  return images
}

const screenshotLine = /^Screenshot "(.+)" at (\d+(?:\.\d+)?)px$/gm

export function toolImageCaptions(result: any): string[] {
  const content = typeof result?.content === 'string' ? result.content : ''
  return [...content.matchAll(screenshotLine)].map((match) => `${match[1]} · ${match[2]}px`)
}

export function ToolGallery({ images, captions = [] }: { images: ToolImage[]; captions?: string[] }) {
  const { t } = useI18n()
  const [zoomed, setZoomed] = useState(-1)
  if (!images.length) return null
  const multi = images.length > 1
  const source = (image: ToolImage) => `data:${image.mime_type};base64,${image.data}`
  const label = (index: number) => captions[index] || images[index].path || ''
  const current = images[zoomed]
  return (
    <>
      <div className={`tool-gallery ${multi ? 'multi' : ''}`}>
        {multi && <div className="tool-gallery-count">{t('Снимков: {count}', { count: images.length })}</div>}
        {images.map((image, index) => (
          <button
            key={index}
            className="tool-image"
            onClick={() => setZoomed(index)}
            aria-label={label(index) || t('Изображение')}
            title={label(index)}
          >
            <img src={source(image)} alt={label(index)} loading="lazy" decoding="async" />
            {multi && label(index) && <span className="tool-image-caption">{label(index)}</span>}
          </button>
        ))}
      </div>
      {current && (
        <ImageLightbox
          src={source(current)}
          alt={label(zoomed)}
          caption={multi ? `${zoomed + 1} / ${images.length}${label(zoomed) ? ` · ${label(zoomed)}` : ''}` : undefined}
          close={() => setZoomed(-1)}
          previous={multi ? () => setZoomed((zoomed - 1 + images.length) % images.length) : undefined}
          next={multi ? () => setZoomed((zoomed + 1) % images.length) : undefined}
        />
      )}
    </>
  )
}

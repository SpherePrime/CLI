import { useState } from 'react'
import { ImageLightbox } from '../../components/ImageLightbox'

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
  for (const image of Array.isArray(result.images) ? result.images : []) {
    if (
      typeof image?.mime_type === 'string' &&
      image.mime_type.startsWith('image/') &&
      typeof image?.data === 'string' &&
      image.data
    )
      images.push({ data: image.data, mime_type: image.mime_type, path: image.path })
  }
  return images
}

export function ToolGallery({ images }: { images: ToolImage[] }) {
  const [zoomed, setZoomed] = useState(-1)
  if (!images.length) return null
  return (
    <>
      <div className={`tool-gallery ${images.length > 1 ? 'multi' : ''}`}>
        {images.map((image, index) => (
          <button
            key={index}
            className="tool-image"
            onClick={() => setZoomed(index)}
            aria-label={image.path || ''}
            title={image.path}
          >
            <img src={`data:${image.mime_type};base64,${image.data}`} alt={image.path || ''} />
          </button>
        ))}
      </div>
      {zoomed >= 0 && images[zoomed] && (
        <ImageLightbox
          src={`data:${images[zoomed].mime_type};base64,${images[zoomed].data}`}
          alt={images[zoomed].path || ''}
          close={() => setZoomed(-1)}
        />
      )}
    </>
  )
}

import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { ToolGallery, collectToolImages, toolImageCaptions } from '../../src/renderer/src/features/chat/ToolGallery'

describe('ToolGallery compact overflow', () => {
  it('shows at most 4 thumbnails with +N badge when more than 4 images exist', () => {
    const images = Array.from({ length: 7 }, (_, i) => ({
      data: `IMG_${i}`,
      mime_type: 'image/png'
    }))
    const captions = images.map((_, i) => `shot-${i + 1} · 1280px`)
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ToolGallery images={images} captions={captions} />
      </I18nProvider>
    )
    expect(html).toContain('Снимков: 7')
    expect((html.match(/<img /g) || []).length).toBe(4)
    expect(html).toContain('+3')
    expect(html).toContain('shot-1 · 1280px')
    expect(html).not.toContain('shot-5 · 1280px')
  })

  it('does not show +N overlay when 4 or fewer images exist', () => {
    const images = Array.from({ length: 4 }, (_, i) => ({
      data: `IMG_${i}`,
      mime_type: 'image/png'
    }))
    const html = renderToStaticMarkup(
      <I18nProvider language="en">
        <ToolGallery images={images} />
      </I18nProvider>
    )
    expect(html).toContain('Screenshots: 4')
    expect((html.match(/<img /g) || []).length).toBe(4)
    expect(html).not.toContain('tool-image-more')
  })

  it('deduplicates images and extracts browser captions', () => {
    const imgs = collectToolImages({
      data: 'AAA',
      mime_type: 'image/png',
      images: [
        { data: 'AAA', mime_type: 'image/png' },
        { data: 'BBB', mime_type: 'image/png' }
      ]
    })
    expect(imgs.map((x) => x.data)).toEqual(['AAA', 'BBB'])
    expect(toolImageCaptions({ content: 'Screenshot "hero" at 1440px' })).toEqual(['hero · 1440px'])
  })
})

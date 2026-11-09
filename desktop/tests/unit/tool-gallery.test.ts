import { describe, expect, it } from 'vitest'
import { collectToolImages, toolImageCaptions } from '../../src/renderer/src/features/chat/ToolGallery'

describe('tool gallery data', () => {
  it('lists every image once when the first is mirrored into data', () => {
    const result = {
      data: 'AAA',
      mime_type: 'image/png',
      images: [
        { data: 'AAA', mime_type: 'image/png' },
        { data: 'BBB', mime_type: 'image/png' },
        { data: 'CCC', mime_type: 'image/png' }
      ]
    }
    expect(collectToolImages(result).map((image) => image.data)).toEqual(['AAA', 'BBB', 'CCC'])
  })

  it('reads screenshot captions from browser_run output', () => {
    const content =
      'Scenario PASSED: 4 of 4 steps run\n1. navigate ok 40ms\nScreenshot "home desktop" at 1280px\nScreenshot "menu open" at 390px'
    expect(toolImageCaptions({ content })).toEqual(['home desktop · 1280px', 'menu open · 390px'])
    expect(toolImageCaptions({})).toEqual([])
  })
})

import { expect, it } from 'vitest'
import { importImages } from '../../src/main/attachments/images'
import type { StoredAttachment } from '../../src/shared/attachments'

const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j1ioAAAAASUVORK5CYII=',
  'base64'
)
it('stores pasted image bytes and returns a thumbnail without trusting its file name', () => {
  const store = new Map<string, StoredAttachment>()
  const result = importImages(
    store,
    [{ name: '../../screenshot.png', data: png }],
    () => 'data:image/png;base64,preview'
  )
  expect(result[0]).toMatchObject({
    mimeType: 'image/png',
    size: png.length,
    preview: 'data:image/png;base64,preview'
  })
  expect(result[0].name).not.toMatch(/[\\/]/)
  expect(store.get(result[0].id)?.content).toBe(png.toString('base64'))
})
it('rejects invalid images and oversize batches without partly storing attachments', () => {
  const store = new Map<string, StoredAttachment>()
  expect(() =>
    importImages(
      store,
      [
        { name: 'valid.png', data: png },
        { name: 'fake.png', data: Buffer.from('not an image') }
      ],
      () => ''
    )
  ).toThrow()
  expect(store.size).toBe(0)
  expect(() =>
    importImages(store, [{ name: 'big.png', data: Buffer.alloc(20 * 1024 * 1024 + 1) }], () => '')
  ).toThrow('20 MiB')
})

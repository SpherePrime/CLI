import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import test from 'node:test'
import sharp from 'sharp'

const root = new URL('../../', import.meta.url)
const sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256]
const asset = (name) => new URL(`build/${name}`, root)

test('desktop PNG is a reproducible RGBA rendering of the source mark', async () => {
  const png = await readFile(asset('icon.png'))
  const metadata = await sharp(png).metadata()
  assert.equal(metadata.width, 512)
  assert.equal(metadata.height, 512)
  assert.equal(metadata.channels, 4)
  assert.deepEqual(png, await sharp(fileURLToPath(asset('icon.svg'))).resize(512, 512).png().toBuffer())
})

test('Windows icon contains matching native-size frames including taskbar sizes', async () => {
  const ico = await readFile(asset('icon.ico'))
  assert.equal(ico.readUInt16LE(0), 0)
  assert.equal(ico.readUInt16LE(2), 1)
  assert.equal(ico.readUInt16LE(4), sizes.length)
  let expectedOffset = 6 + sizes.length * 16
  for (const [index, size] of sizes.entries()) {
    const entry = 6 + index * 16
    assert.equal(ico[entry], size % 256)
    assert.equal(ico[entry + 1], size % 256)
    assert.equal(ico.readUInt16LE(entry + 4), 1)
    assert.equal(ico.readUInt16LE(entry + 6), 32)
    const length = ico.readUInt32LE(entry + 8)
    const offset = ico.readUInt32LE(entry + 12)
    assert.equal(offset, expectedOffset)
    const frame = ico.subarray(offset, offset + length)
    assert.deepEqual(frame, await sharp(fileURLToPath(asset('icon.svg'))).resize(size, size).png().toBuffer())
    expectedOffset += length
  }
  assert.equal(expectedOffset, ico.length)
})

test('native application, setup and NSIS share the desktop icon assets', async () => {
  for (const configPath of ['src-tauri/tauri.conf.json', 'installer/src-tauri/tauri.conf.json']) {
    const url = new URL(configPath, root)
    const config = JSON.parse(await readFile(url, 'utf8'))
    assert.deepEqual(config.bundle.icon.map((path) => new URL(path, url).href), [asset('icon.png').href, asset('icon.ico').href])
    if (config.bundle.windows?.nsis) {
      assert.equal(new URL(config.bundle.windows.nsis.installerIcon, url).href, asset('icon.ico').href)
    }
  }
})

import sharp from 'sharp'
import { writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'

const source = fileURLToPath(new URL('../build/icon.svg', import.meta.url))
const sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256]
const render = (size) => sharp(source).resize(size, size).png().toBuffer()
const frames = await Promise.all(sizes.map(render))
const directory = Buffer.alloc(6 + sizes.length * 16)
directory.writeUInt16LE(1, 2)
directory.writeUInt16LE(sizes.length, 4)
let offset = directory.length
for (const [index, size] of sizes.entries()) {
  const entry = 6 + index * 16
  directory[entry] = size % 256
  directory[entry + 1] = size % 256
  directory.writeUInt16LE(1, entry + 4)
  directory.writeUInt16LE(32, entry + 6)
  directory.writeUInt32LE(frames[index].length, entry + 8)
  directory.writeUInt32LE(offset, entry + 12)
  offset += frames[index].length
}
await writeFile(new URL('../build/icon.png', import.meta.url), await render(512))
await writeFile(new URL('../build/icon.ico', import.meta.url), Buffer.concat([directory, ...frames]))
console.log('Prime Desktop icons built')

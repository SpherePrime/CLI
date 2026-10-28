import { readdir, mkdir, copyFile, readFile, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
const tag = process.argv[2]
if (!/^v\d+\.\d+\.\d+$/.test(tag || '')) throw new Error('A stable release tag is required')
const destination = 'release/native'
const version = tag.slice(1)
const platform = process.argv[3] || (process.platform === 'win32' ? 'windows-x86_64' : 'linux-x86_64')
if (!['windows-x86_64', 'linux-x86_64'].includes(platform)) throw new Error(`Unsupported platform: ${platform}`)
await mkdir(destination, { recursive: true })
const platforms = {}
const assets = new Set()
const visit = async (directory) => {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) await visit(path)
    else if (/\.(exe|AppImage|deb|rpm|sig)$/.test(entry.name)) {
      const published = entry.name.replaceAll(' ', '.')
      if (published.startsWith('Prime.Desktop.Setup_')) continue
      if (!published.includes(`_${version}_`) && !published.includes(`-${version}.`) && !published.includes(`-${version}-`)) continue
      await copyFile(path, join(destination, published))
      assets.add(published)
      if (published.endsWith('.sig')) {
        const asset = published.slice(0, -4)
        await readFile(join(directory, entry.name.slice(0, -4)))
        const target = asset === `PrimeDesktop-Setup-${version}.exe` ? 'windows-x86_64'
          : asset.startsWith('PrimeDesktop-Setup-') ? null
          : asset.endsWith('.AppImage') ? 'linux-x86_64-appimage'
          : asset.endsWith('.deb') ? 'linux-x86_64-deb'
          : asset.endsWith('.rpm') ? 'linux-x86_64-rpm' : null
        if (target) platforms[target] = {
          signature: (await readFile(path, 'utf8')).trim(),
          url: `https://github.com/SpherePrime/CLI/releases/download/${tag}/${encodeURIComponent(asset)}`
        }
      }
    }
  }
}
await visit('src-tauri/target/release/bundle')
if (platform === 'windows-x86_64' && !platforms[platform]) throw new Error('Missing signed custom setup for Windows')
if (platform === 'linux-x86_64') {
  if (!assets.has(`PrimeDesktop-Setup-${version}.AppImage`)) throw new Error('Missing custom setup for Linux')
  for (const type of ['appimage', 'deb', 'rpm']) {
    if (!platforms[`${platform}-${type}`]) throw new Error(`Missing signed ${type} updater artifact for Linux`)
  }
  platforms[platform] = platforms[`${platform}-appimage`]
}
await writeFile(join(destination, `${platform}.json`), JSON.stringify(platforms))

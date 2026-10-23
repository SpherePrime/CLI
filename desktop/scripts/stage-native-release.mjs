import { readdir, mkdir, copyFile, readFile, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
const tag = process.argv[2]
if (!/^v\d+\.\d+\.\d+$/.test(tag || '')) throw new Error('A stable release tag is required')
const destination = 'release/native'
await mkdir(destination, { recursive: true })
const platforms = {}
const visit = async (directory) => {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) await visit(path)
    else if (/\.(exe|AppImage|deb|rpm|sig)$/.test(entry.name)) {
      const published = entry.name.replaceAll(' ', '.')
      await copyFile(path, join(destination, published))
      if (published.endsWith('.sig')) {
        const asset = published.slice(0, -4)
        const platform = asset.endsWith('.exe') ? 'windows-x86_64' : asset.endsWith('.AppImage') ? 'linux-x86_64' : null
        if (platform) platforms[platform] = {
          signature: (await readFile(path, 'utf8')).trim(),
          url: `https://github.com/SpherePrime/CLI/releases/download/${tag}/${encodeURIComponent(asset)}`
        }
      }
    }
  }
}
await visit('src-tauri/target/release/bundle')
const platform = process.platform === 'win32' ? 'windows-x86_64' : 'linux-x86_64'
if (!platforms[platform]) throw new Error(`Missing signed updater artifact for ${platform}`)
await writeFile(join(destination, `${platform}.json`), JSON.stringify(platforms))

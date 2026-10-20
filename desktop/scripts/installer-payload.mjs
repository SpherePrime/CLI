import { createHash } from 'node:crypto'
import { createReadStream } from 'node:fs'
import { copyFile, readdir, stat } from 'node:fs/promises'
import { join } from 'node:path'

export async function sha256(path) {
  const hash = createHash('sha256')
  for await (const bytes of createReadStream(path)) hash.update(bytes)
  return hash.digest('hex')
}

export async function directorySize(directory) {
  let size = 0
  for (const entry of await readdir(directory, { recursive: true, withFileTypes: true }))
    if (entry.isFile()) size += (await stat(join(entry.parentPath, entry.name))).size
  return size
}

export async function windowsPayload({ release, payload, version, runtime }) {
  const source = join(release, `PrimeDesktop-Update-${version}.exe`)
  await copyFile(source, join(payload, 'PrimeDesktopSetup.exe'))
  return {
    version,
    sha256: await sha256(source),
    runtimeSha256: runtime.sha256,
    installedSize: await directorySize(join(release, 'win-unpacked'))
  }
}

const linuxPackages = { deb: 'deb', rpm: 'rpm', pacman: 'pacman', appimage: 'AppImage' }

export async function linuxPayload({ release, payload, version, runtime, online }) {
  const packages = {}
  for (const [kind, extension] of Object.entries(linuxPackages)) {
    const file = `PrimeDesktop-${version}.${extension}`
    const source = join(release, file)
    packages[kind] = { file, sha256: await sha256(source), size: (await stat(source)).size }
    if (!online) await copyFile(source, join(payload, file))
  }
  return {
    platform: 'linux',
    version,
    runtimeSha256: runtime.sha256,
    installedSize: await directorySize(join(release, 'linux-unpacked')),
    packages
  }
}

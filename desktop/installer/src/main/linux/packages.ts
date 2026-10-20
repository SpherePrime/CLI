import { createWriteStream, existsSync } from 'node:fs'
import { chmod, copyFile, mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { Readable } from 'node:stream'
import { pipeline } from 'node:stream/promises'
import { checksum } from '../checksum'
import type { LinuxPackageKind } from './distribution'

export type PackageEntry = { file: string; sha256: string; size: number }
export type LinuxPayload = {
  version: string
  runtimeSha256: string
  installedSize: number
  packages: Record<LinuxPackageKind, PackageEntry>
}
const kinds: LinuxPackageKind[] = ['deb', 'rpm', 'pacman', 'appimage']
const releaseBase = 'https://github.com/SpherePrime/CLI/releases/download'

export function readLinuxPayload(value: any): LinuxPayload {
  const valid =
    value?.platform === 'linux' &&
    /^\d+\.\d+\.\d+$/.test(value.version) &&
    /^[a-f0-9]{64}$/.test(value.runtimeSha256) &&
    kinds.every((kind) => {
      const entry = value.packages?.[kind]
      return (
        typeof entry?.file === 'string' &&
        /^[\w.-]+$/.test(entry.file) &&
        /^[a-f0-9]{64}$/.test(entry.sha256)
      )
    })
  if (!valid) throw new Error('Повреждена конфигурация установщика')
  return value
}

export async function stagePackage(root: string, payload: LinuxPayload, kind: LinuxPackageKind) {
  const entry = payload.packages[kind]
  const directory = await mkdtemp(join(tmpdir(), 'prime-setup-'))
  await chmod(directory, 0o755)
  const target = join(directory, entry.file)
  const bundled = join(root, entry.file)
  try {
    if (existsSync(bundled)) await copyFile(bundled, target)
    else {
      const response = await fetch(`${releaseBase}/v${payload.version}/${entry.file}`, {
        headers: { 'User-Agent': 'SpherePrime-Setup' }
      })
      if (!response.ok || !response.body)
        throw new Error(`Не удалось скачать Prime Desktop (HTTP ${response.status})`)
      await pipeline(Readable.fromWeb(response.body as any), createWriteStream(target))
    }
    await chmod(target, 0o644)
    if ((await checksum(target)) !== entry.sha256)
      throw new Error('Файл установки повреждён. Скачайте установщик заново.')
    return { path: target, cleanup: () => rm(directory, { recursive: true, force: true }) }
  } catch (error) {
    await rm(directory, { recursive: true, force: true })
    throw error
  }
}

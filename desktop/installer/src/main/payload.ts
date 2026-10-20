import { readFile } from 'node:fs/promises'
import { join } from 'node:path'
import type { PlatformInstaller } from './platform'
import { readWindowsPayload, windowsInstaller } from './windows/installer'
import { linuxInstaller } from './linux/installer'
import { readLinuxPayload } from './linux/packages'

export async function createInstaller(root: string, icon: string): Promise<PlatformInstaller> {
  const manifest = JSON.parse(await readFile(join(root, 'manifest.json'), 'utf8'))
  if (process.platform === 'linux') return linuxInstaller(root, readLinuxPayload(manifest), icon)
  return windowsInstaller(root, readWindowsPayload(manifest))
}

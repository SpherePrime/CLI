import { constants } from 'node:fs'
import { access, readFile } from 'node:fs/promises'
import { delimiter, join } from 'node:path'

export type LinuxPackageKind = 'deb' | 'rpm' | 'pacman' | 'appimage'
export type LinuxPlan = {
  kind: LinuxPackageKind
  distribution: string
  command?: string[]
}

export async function findCommand(name: string): Promise<string | undefined> {
  const directories = (process.env.PATH || '/usr/local/bin:/usr/bin:/bin').split(delimiter)
  for (const directory of [...directories, '/usr/sbin', '/sbin']) {
    const candidate = join(directory, name)
    try {
      await access(candidate, constants.X_OK)
      return candidate
    } catch {}
  }
  return undefined
}

export function parseOsRelease(text: string): Record<string, string> {
  const values: Record<string, string> = {}
  for (const line of text.split(/\r?\n/)) {
    const match = /^([A-Z_]+)=(.*)$/.exec(line.trim())
    if (match) values[match[1]] = match[2].replace(/^(["'])(.*)\1$/, '$2')
  }
  return values
}

async function osRelease() {
  for (const path of ['/etc/os-release', '/usr/lib/os-release']) {
    try {
      return parseOsRelease(await readFile(path, 'utf8'))
    } catch {}
  }
  return {}
}

export function selectPackageKind(
  release: Record<string, string>,
  available: (name: string) => boolean
): Omit<LinuxPlan, 'distribution'> {
  const families = `${release.ID || ''} ${release.ID_LIKE || ''}`.toLowerCase().split(/\s+/)
  const family = (...names: string[]) => names.some((name) => families.includes(name))
  if (!available('pkexec')) return { kind: 'appimage' }
  const apt = available('apt-get') && available('dpkg')
  const pacman = available('pacman')
  const dnf = available('dnf')
  const zypper = available('zypper')
  if (family('debian', 'ubuntu') && apt)
    return { kind: 'deb', command: ['apt-get', 'install', '-y', '--allow-downgrades'] }
  if (family('arch') && pacman) return { kind: 'pacman', command: ['pacman', '-U', '--noconfirm'] }
  if (family('fedora', 'rhel', 'centos') && dnf)
    return { kind: 'rpm', command: ['dnf', 'install', '-y'] }
  if (family('suse', 'opensuse') && zypper)
    return {
      kind: 'rpm',
      command: ['zypper', '--non-interactive', 'install', '--allow-unsigned-rpm']
    }
  if (apt) return { kind: 'deb', command: ['apt-get', 'install', '-y', '--allow-downgrades'] }
  if (pacman) return { kind: 'pacman', command: ['pacman', '-U', '--noconfirm'] }
  if (dnf) return { kind: 'rpm', command: ['dnf', 'install', '-y'] }
  if (zypper)
    return {
      kind: 'rpm',
      command: ['zypper', '--non-interactive', 'install', '--allow-unsigned-rpm']
    }
  return { kind: 'appimage' }
}

export async function detectPlan(): Promise<LinuxPlan> {
  const release = await osRelease()
  const commands = ['pkexec', 'apt-get', 'dpkg', 'pacman', 'dnf', 'zypper']
  const found = new Set<string>()
  for (const name of commands) if (await findCommand(name)) found.add(name)
  return {
    ...selectPackageKind(release, (name) => found.has(name)),
    distribution: release.PRETTY_NAME || release.NAME || 'Linux'
  }
}

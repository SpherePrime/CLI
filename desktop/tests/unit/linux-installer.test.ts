import { expect, it } from 'vitest'
import { parseOsRelease, selectPackageKind } from '../../installer/src/main/linux/distribution'
import { readLinuxPayload } from '../../installer/src/main/linux/packages'

const tools =
  (...names: string[]) =>
  (name: string) =>
    names.includes(name)

it('picks the native package for the detected distribution family', () => {
  const ubuntu = parseOsRelease('ID=ubuntu\nID_LIKE=debian\nPRETTY_NAME="Ubuntu 24.04 LTS"')
  expect(ubuntu.PRETTY_NAME).toBe('Ubuntu 24.04 LTS')
  expect(selectPackageKind(ubuntu, tools('pkexec', 'apt-get', 'dpkg')).kind).toBe('deb')
  expect(
    selectPackageKind({ ID: 'manjaro', ID_LIKE: 'arch' }, tools('pkexec', 'pacman')).kind
  ).toBe('pacman')
  expect(selectPackageKind({ ID: 'fedora' }, tools('pkexec', 'dnf')).command?.[0]).toBe('dnf')
  expect(
    selectPackageKind(
      { ID: 'opensuse-tumbleweed', ID_LIKE: 'opensuse suse' },
      tools('pkexec', 'zypper')
    ).command?.[0]
  ).toBe('zypper')
})

it('falls back to a per-user AppImage without a supported package manager or pkexec', () => {
  expect(selectPackageKind({ ID: 'ubuntu' }, tools('apt-get', 'dpkg')).kind).toBe('appimage')
  expect(selectPackageKind({ ID: 'nixos' }, tools('pkexec')).kind).toBe('appimage')
  expect(selectPackageKind({ ID: 'unknown' }, tools('pkexec', 'pacman')).kind).toBe('pacman')
})

it('rejects Linux payload manifests with unsafe package names', () => {
  const entry = (file: string) => ({ file, sha256: 'a'.repeat(64), size: 1 })
  const manifest = {
    platform: 'linux',
    version: '1.2.3',
    runtimeSha256: 'b'.repeat(64),
    installedSize: 1,
    packages: {
      deb: entry('PrimeDesktop-1.2.3.deb'),
      rpm: entry('PrimeDesktop-1.2.3.rpm'),
      pacman: entry('PrimeDesktop-1.2.3.pacman'),
      appimage: entry('PrimeDesktop-1.2.3.AppImage')
    }
  }
  expect(readLinuxPayload(manifest).version).toBe('1.2.3')
  expect(() =>
    readLinuxPayload({ ...manifest, packages: { ...manifest.packages, deb: entry('../x.deb') } })
  ).toThrow()
})

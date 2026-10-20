const desktopTargets = {
  'win32-x64': { binary: 'prime.exe', goos: 'windows', goarch: 'amd64' },
  'linux-x64': { binary: 'prime', goos: 'linux', goarch: 'amd64' },
  'linux-arm64': { binary: 'prime', goos: 'linux', goarch: 'arm64' }
}

const [desktopPlatform, desktopArch] = (
  process.env.PRIME_DESKTOP_TARGET || `${process.platform}-${process.arch}`
).split('-')
const desktopTarget = desktopTargets[`${desktopPlatform}-${desktopArch}`]
if (!desktopTarget) throw new Error(`Unsupported Desktop target: ${desktopPlatform}-${desktopArch}`)

export const runtimeTargets = [
  { directory: '', platform: desktopPlatform, arch: desktopArch, ...desktopTarget },
  {
    directory: 'remotes/linux-x64',
    binary: 'prime',
    platform: 'linux',
    arch: 'x64',
    goos: 'linux',
    goarch: 'amd64'
  },
  {
    directory: 'remotes/linux-arm64',
    binary: 'prime',
    platform: 'linux',
    arch: 'arm64',
    goos: 'linux',
    goarch: 'arm64'
  }
]

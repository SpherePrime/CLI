export const runtimeTargets = [
  {
    directory: '',
    binary: 'prime.exe',
    platform: 'win32',
    arch: 'x64',
    goos: 'windows',
    goarch: 'amd64'
  },
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

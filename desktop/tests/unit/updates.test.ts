import { EventEmitter } from 'node:events'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { UpdateController } from '../../src/main/updates/controller'
import {
  discoverDesktopRelease,
  releaseAssets,
  selectDesktopRelease
} from '../../src/main/updates/github'

const windowsAssets = releaseAssets('win32')

const release = (version: string, extra = {}) => ({
  tag_name: `v${version}`,
  draft: false,
  prerelease: false,
  assets: [{ name: 'latest.yml' }, { name: `Prime-Desktop-Setup-${version}.exe` }],
  ...extra
})
describe('release updates', () => {
  afterEach(() => vi.unstubAllGlobals())
  it('discovers complete releases through the public feed without requiring the GitHub API', async () => {
    const request = vi.fn(async (url: string) => {
      if (url.endsWith('.atom'))
        return new Response(
          '<feed><entry><link href="https://github.com/SpherePrime/CLI/releases/tag/v0.4.53"/></entry><entry><link href="https://github.com/SpherePrime/CLI/releases/tag/v0.4.52"/></entry></feed>'
        )
      if (url.includes('v0.4.53')) return new Response('', { status: 404 })
      if (url.endsWith('latest.yml')) return new Response('version: 0.4.52\n')
      return new Response('')
    })
    vi.stubGlobal('fetch', request)
    expect(await discoverDesktopRelease('0.4.51', windowsAssets)).toMatchObject({ version: '0.4.52' })
    expect(request.mock.calls.every(([url]) => !url.includes('api.github.com'))).toBe(true)
  })
  it('selects only a newer stable release with complete desktop assets', () => {
    expect(
      selectDesktopRelease(
        [release('0.4.53', { assets: [] }), release('0.4.52'), release('0.4.51')],
        '0.4.51',
        windowsAssets
      )?.version
    ).toBe('0.4.52')
    expect(
      selectDesktopRelease(
        [release('0.4.52', { prerelease: true }), release('0.4.50')],
        '0.4.51',
        windowsAssets
      )
    ).toBeNull()
  })
  it('selects Linux releases by AppImage update metadata', () => {
    const linux = releaseAssets('linux')
    const linuxRelease = (version: string) =>
      release(version, {
        assets: [{ name: 'latest-linux.yml' }, { name: linux.binary(version) }]
      })
    expect(
      selectDesktopRelease([release('0.4.53'), linuxRelease('0.4.52')], '0.4.51', linux)?.version
    ).toBe('0.4.52')
  })
  it('downloads once and installs only after download and safe shutdown', async () => {
    const updater = Object.assign(new EventEmitter(), {
      autoDownload: true,
      autoInstallOnAppQuit: true,
      setFeedURL: vi.fn(),
      checkForUpdates: vi.fn(async () => ({ isUpdateAvailable: true })),
      downloadUpdate: vi.fn(async () => {
        updater.emit('download-progress', {
          percent: 37,
          transferred: 37,
          total: 100,
          bytesPerSecond: 9
        })
        updater.emit('update-downloaded', { version: '0.4.52' })
      }),
      quitAndInstall: vi.fn()
    })
    const prepare = vi.fn(async () => {})
    const discover = vi.fn(async () => ({
      version: '0.4.52',
      url: 'https://github.com/SpherePrime/CLI/releases/download/v0.4.52/'
    }))
    const states: string[] = []
    const controller = new UpdateController(
      updater,
      discover,
      (state) => states.push(state.state),
      prepare
    )
    await expect(controller.install()).rejects.toThrow()
    await Promise.all([controller.check(), controller.check()])
    expect(discover).toHaveBeenCalledTimes(1)
    expect(updater.downloadUpdate).toHaveBeenCalledTimes(1)
    expect(states).toEqual(['checking', 'available', 'downloading', 'downloaded'])
    expect(updater.autoInstallOnAppQuit).toBe(false)
    expect(updater.quitAndInstall).not.toHaveBeenCalled()
    await controller.install()
    expect(prepare).toHaveBeenCalledTimes(1)
    expect(updater.quitAndInstall).toHaveBeenCalledWith(true, true)
  })
  it('retains a downloaded update when restart is blocked by active work', async () => {
    const updater = Object.assign(new EventEmitter(), {
      autoDownload: false,
      autoInstallOnAppQuit: false,
      setFeedURL: vi.fn(),
      checkForUpdates: vi.fn(),
      downloadUpdate: vi.fn(),
      quitAndInstall: vi.fn()
    })
    const controller = new UpdateController(
      updater,
      async () => null,
      () => {},
      async () => {
        throw new Error('Busy')
      }
    )
    updater.emit('update-downloaded', { version: '0.4.52' })
    await expect(controller.install()).rejects.toThrow('Busy')
    expect(controller.snapshot().state).toBe('downloaded')
    expect(updater.quitAndInstall).not.toHaveBeenCalled()
  })
  it('shows shutdown progress before preparation completes and rejects installer errors', async () => {
    let finishShutdown!: () => void
    const shutdown = new Promise<void>((resolve) => {
      finishShutdown = resolve
    })
    const updater = Object.assign(new EventEmitter(), {
      autoDownload: false,
      autoInstallOnAppQuit: false,
      setFeedURL: vi.fn(),
      checkForUpdates: vi.fn(),
      downloadUpdate: vi.fn(),
      quitAndInstall: () => {
        updater.emit('error', new Error('Installation failed'))
      }
    })
    const controller = new UpdateController(
      updater,
      async () => null,
      () => {},
      () => shutdown
    )
    updater.emit('update-downloaded', { version: '0.4.52' })
    const installing = controller.install()
    const stateDuringShutdown = controller.snapshot().state
    finishShutdown()
    await expect(installing).rejects.toThrow('Installation failed')
    expect(stateDuringShutdown).toBe('installing')
    expect(controller.snapshot()).toMatchObject({ state: 'error', message: 'Installation failed' })
  })
  it('recovers from a failed download without treating it as ready to install', async () => {
    const updater = Object.assign(new EventEmitter(), {
      autoDownload: false,
      autoInstallOnAppQuit: false,
      setFeedURL: vi.fn(),
      checkForUpdates: vi.fn(async () => ({ isUpdateAvailable: true })),
      downloadUpdate: vi.fn(async () => {
        throw new Error('Network unavailable')
      }),
      quitAndInstall: vi.fn()
    })
    const controller = new UpdateController(
      updater,
      async () => ({
        version: '0.4.52',
        url: 'https://github.com/SpherePrime/CLI/releases/download/v0.4.52/'
      }),
      () => {},
      async () => {}
    )
    await controller.check()
    expect(controller.snapshot()).toMatchObject({ state: 'error', version: '0.4.52' })
    await expect(controller.install()).rejects.toThrow()
  })
})

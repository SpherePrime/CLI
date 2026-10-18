import type { UpdateStatus } from '../../shared/updates'
import type { DesktopRelease } from './github'

type Updater = {
  autoDownload: boolean
  autoInstallOnAppQuit: boolean
  on(event: string, handler: (...args: any[]) => void): unknown
  setFeedURL(options: { provider: 'generic'; url: string }): void
  checkForUpdates(): Promise<{ isUpdateAvailable: boolean } | null>
  downloadUpdate(): Promise<unknown>
  quitAndInstall(silent: boolean, relaunch: boolean): void
}
export class UpdateController {
  private status: UpdateStatus = { state: 'idle' }
  private pending: Promise<UpdateStatus> | undefined
  private installing = false
  constructor(
    private updater: Updater,
    private discover: () => Promise<DesktopRelease | null>,
    private emit: (status: UpdateStatus) => void,
    private prepare: () => Promise<void>
  ) {
    updater.autoDownload = false
    updater.autoInstallOnAppQuit = false
    updater.on('error', (error) =>
      this.set({ state: 'error', message: error.message || String(error) })
    )
    updater.on('download-progress', (progress) =>
      this.set({
        state: 'downloading',
        percent: Math.max(0, Math.min(100, progress.percent || 0)),
        transferred: progress.transferred,
        total: progress.total,
        bytesPerSecond: progress.bytesPerSecond
      })
    )
    updater.on('update-downloaded', (info) =>
      this.set({ state: 'downloaded', version: info.version, percent: 100 })
    )
  }
  snapshot(): UpdateStatus {
    return { ...this.status }
  }
  private set(patch: Partial<UpdateStatus>) {
    this.status = { ...this.status, ...patch }
    this.emit(this.snapshot())
  }
  check(): Promise<UpdateStatus> {
    if (this.pending) return this.pending
    if (['downloaded', 'installing'].includes(this.status.state))
      return Promise.resolve(this.snapshot())
    this.pending = this.performCheck().finally(() => {
      this.pending = undefined
    })
    return this.pending
  }
  private async performCheck(): Promise<UpdateStatus> {
    this.status = { state: 'checking' }
    this.emit(this.snapshot())
    try {
      const release = await this.discover()
      if (!release) this.set({ state: 'current' })
      else {
        this.updater.setFeedURL({ provider: 'generic', url: release.url })
        const result = await this.updater.checkForUpdates()
        if (!result?.isUpdateAvailable) this.set({ state: 'current' })
        else {
          this.set({ state: 'available', version: release.version, percent: 0 })
          await this.updater.downloadUpdate()
        }
      }
    } catch (error: any) {
      this.set({ state: 'error', message: error.message || String(error) })
    }
    return this.snapshot()
  }
  async install(): Promise<UpdateStatus> {
    if (this.status.state !== 'downloaded' || this.installing)
      throw new Error('Update is not ready to install')
    this.installing = true
    try {
      await this.prepare()
      this.set({ state: 'installing' })
      this.updater.quitAndInstall(true, true)
      return this.snapshot()
    } finally {
      this.installing = false
    }
  }
}

import { posix, win32 } from 'node:path'
import type { InstallStatus } from '../shared/contracts'
import type { InstallOperations } from './platform'

export class InstallerController {
  private status: InstallStatus = { phase: 'ready' }
  constructor(
    private operations: InstallOperations,
    private emit: (status: InstallStatus) => void,
    private paths = process.platform === 'win32' ? win32 : posix
  ) {}
  get active() {
    return ['preparing', 'installing', 'verifying', 'voice'].includes(this.status.phase)
  }
  snapshot() {
    return { ...this.status }
  }
  private set(status: InstallStatus) {
    this.status = status
    this.emit(this.snapshot())
  }
  async install(directory: string): Promise<InstallStatus> {
    if (this.active) throw new Error('Установка уже выполняется')
    const paths = this.paths
    if (
      !paths.isAbsolute(directory) ||
      /["\r\n\0]/.test(directory) ||
      paths.parse(directory).root === paths.normalize(directory)
    )
      throw new Error('Выберите папку для приложения')
    directory = paths.normalize(directory)
    try {
      this.set({ phase: 'preparing', directory })
      await this.operations.verifyPayload()
      this.set({ phase: 'installing', directory })
      const notice = (await this.operations.launch(directory))?.notice
      this.set({ phase: 'verifying', directory })
      await this.operations.verifyInstall(directory)
      this.set({ phase: 'voice', directory, percent: 0 })
      let voiceWarning: string | undefined
      let reported = 0
      await this.operations
        .prepareVoice((percent) => {
          if (percent - reported < 0.5 && percent < 100) return
          reported = percent
          this.set({ phase: 'voice', directory, percent })
        })
        .catch(() => {
          voiceWarning =
            'Голосовая модель не скачалась. Prime докачает её при первом голосовом разговоре.'
        })
      const warning = [notice, voiceWarning].filter(Boolean).join(' ')
      this.set({ phase: 'complete', directory, ...(warning ? { warning } : {}) })
    } catch (error: any) {
      this.set({ phase: 'error', directory, error: error.message })
    }
    return this.snapshot()
  }
}

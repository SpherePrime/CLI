import { win32 } from 'node:path'
import type { InstallStatus } from '../shared/contracts'

type Operations = {
  verifyPayload(): Promise<void>
  launch(directory: string): Promise<void>
  verifyInstall(directory: string): Promise<void>
}
export class InstallerController {
  private status: InstallStatus = { phase: 'ready' }
  constructor(
    private operations: Operations,
    private emit: (status: InstallStatus) => void
  ) {}
  get active() {
    return ['preparing', 'installing', 'verifying'].includes(this.status.phase)
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
    if (
      !win32.isAbsolute(directory) ||
      /["\r\n\0]/.test(directory) ||
      win32.parse(directory).root === win32.normalize(directory)
    )
      throw new Error('Выберите папку для приложения')
    directory = win32.normalize(directory)
    try {
      this.set({ phase: 'preparing', directory })
      await this.operations.verifyPayload()
      this.set({ phase: 'installing', directory })
      await this.operations.launch(directory)
      this.set({ phase: 'verifying', directory })
      await this.operations.verifyInstall(directory)
      this.set({ phase: 'complete', directory })
    } catch (error: any) {
      this.set({ phase: 'error', directory, error: error.message })
    }
    return this.snapshot()
  }
}

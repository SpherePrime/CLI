export type InstallPhase = 'ready' | 'preparing' | 'installing' | 'verifying' | 'complete' | 'error'
export type InstallStatus = { phase: InstallPhase; error?: string; directory?: string }
export interface SetupApi {
  info(): Promise<{ version: string; directory: string; installedSize: number }>
  chooseDirectory(current: string): Promise<string | null>
  install(directory: string): Promise<InstallStatus>
  open(): Promise<void>
  close(): Promise<void>
  onProgress(listener: (status: InstallStatus) => void): () => void
}
declare global {
  interface Window {
    setup: SetupApi
  }
}

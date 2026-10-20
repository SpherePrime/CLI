export type InstallPhase =
  'ready' | 'preparing' | 'installing' | 'verifying' | 'voice' | 'complete' | 'error'
export type InstallStatus = {
  phase: InstallPhase
  error?: string
  directory?: string
  percent?: number
  warning?: string
}
export type InstallMethod = { label: string; directoryLocked: boolean }
export type SetupInfo = {
  version: string
  directory: string
  installedSize: number
  method: InstallMethod
}
export interface SetupApi {
  info(): Promise<SetupInfo>
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

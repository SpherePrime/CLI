import type { InstallMethod } from '../shared/contracts'

export type LaunchResult = { notice?: string } | void
export type InstallOperations = {
  verifyPayload(): Promise<void>
  launch(directory: string): Promise<LaunchResult>
  verifyInstall(directory: string): Promise<void>
  prepareVoice(progress: (percent: number) => void): Promise<void>
}
export type PlatformInstaller = {
  version: string
  installedSize: number
  method(): Promise<InstallMethod>
  defaultDirectory(): Promise<string>
  operations: InstallOperations
  open(directory: string): Promise<{ binary: string; args: string[]; env?: NodeJS.ProcessEnv }>
}

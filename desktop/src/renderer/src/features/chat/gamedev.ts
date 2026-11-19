import type { WorkspaceConfig } from '../../../../shared/contracts'

export function resolveGameDevEnabled(
  config: WorkspaceConfig | undefined,
  realSessionId?: string,
  draftGameDev: boolean | null = null
): boolean {
  const sessionFlag = realSessionId ? config?.session_game_dev?.[realSessionId] : undefined
  if (realSessionId) {
    return sessionFlag !== undefined ? Boolean(sessionFlag) : Boolean(config?.options?.game_dev)
  }
  return draftGameDev !== null ? draftGameDev : Boolean(config?.options?.game_dev)
}

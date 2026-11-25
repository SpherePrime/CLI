import type { WorkspaceConfig } from '../../../../shared/contracts'

export function resolveStrictVerifyEnabled(
  config: WorkspaceConfig | undefined,
  realSessionId?: string,
  draftStrictVerify: boolean | null = null
): boolean {
  const sessionFlag = realSessionId ? config?.session_strict_verify?.[realSessionId] : undefined
  if (realSessionId) {
    return sessionFlag !== undefined
      ? Boolean(sessionFlag)
      : Boolean(config?.options?.strict_verify)
  }
  return draftStrictVerify !== null ? draftStrictVerify : Boolean(config?.options?.strict_verify)
}

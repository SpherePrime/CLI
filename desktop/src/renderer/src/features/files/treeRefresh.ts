export function shouldRefreshTreeOnBusyChange(
  previousBusy: boolean,
  nextBusy: boolean,
  previousWorkspaceId: string | undefined,
  nextWorkspaceId: string | undefined
) {
  if (previousWorkspaceId !== nextWorkspaceId) return false
  return previousBusy && !nextBusy
}

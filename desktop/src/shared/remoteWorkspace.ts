export const isRemoteWorkspace = (path: string) => path.startsWith('prime-ssh://')

export function remoteWorkspaceKey(profileId: string, root: string): string {
  return `prime-ssh://${profileId}${root.split('/').map(encodeURIComponent).join('/')}`
}

export function remoteWorkspaceIdentity(path: string): { profileId: string; root: string } | undefined {
  if (!isRemoteWorkspace(path)) return undefined
  try {
    const url = new URL(path)
    if (url.protocol !== 'prime-ssh:' || !/^[a-f0-9-]{36}$/.test(url.hostname) || url.username || url.password || url.port || url.search || url.hash) return undefined
    const root = decodeURIComponent(url.pathname)
    if (!root.startsWith('/') || root.includes('\0')) return undefined
    return { profileId: url.hostname, root }
  } catch { return undefined }
}

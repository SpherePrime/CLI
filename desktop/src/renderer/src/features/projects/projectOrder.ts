export function orderedProjectPaths(recent: string[], open: string[], saved: string[] = []) {
  const available = new Set([...recent, ...open])
  return [...new Set([...saved.filter((path) => available.has(path)), ...available])]
}

export function orderedSessionIds(ids: string[], saved: string[] = []) {
  const available = new Set(ids)
  return [...new Set([...saved.filter((id) => available.has(id)), ...ids])]
}

// Stable chat order: pinned first, then a saved manual order when the user
// has dragged chats, then creation time. Activity never changes a position,
// so a running chat cannot jump to the top while it works.
export function orderedSessions<T extends { id: string; created_at?: number }>(
  sessions: T[],
  pinned: string[],
  saved: string[] = []
): T[] {
  const rank = new Map(saved.map((id, index) => [id, index]))
  return [...sessions].sort((a, b) => {
    const pin = Number(pinned.includes(b.id)) - Number(pinned.includes(a.id))
    if (pin) return pin
    const ra = rank.has(a.id)
    const rb = rank.has(b.id)
    if (ra !== rb) return ra ? -1 : 1
    if (ra && rb) return rank.get(a.id)! - rank.get(b.id)!
    return (b.created_at || 0) - (a.created_at || 0)
  })
}

export function reorderProjectPaths(
  paths: string[],
  source: string,
  target: string,
  position: 'before' | 'after'
) {
  if (source === target || !paths.includes(source) || !paths.includes(target)) return paths
  const reordered = paths.filter((path) => path !== source)
  reordered.splice(reordered.indexOf(target) + Number(position === 'after'), 0, source)
  return reordered
}

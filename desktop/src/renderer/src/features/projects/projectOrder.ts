export function orderedProjectPaths(recent: string[], open: string[], saved: string[] = []) {
  const available = new Set([...recent, ...open])
  return [...new Set([...saved.filter((path) => available.has(path)), ...available])]
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

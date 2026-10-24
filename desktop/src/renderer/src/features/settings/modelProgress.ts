export function progressPercent(done?: number, total?: number): number | undefined {
  if (
    done === undefined ||
    total === undefined ||
    !Number.isFinite(done) ||
    !Number.isFinite(total) ||
    total <= 0
  )
    return undefined
  return Math.max(0, Math.min(100, Math.floor((done / total) * 100)))
}

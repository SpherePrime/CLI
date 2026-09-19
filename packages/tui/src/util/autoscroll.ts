export type AutoscrollState = { atBottom: boolean; newUpdates: number }

export function nearBottom(
  scrollTop: number,
  scrollHeight: number,
  viewHeight: number,
  tolerance = 32,
): boolean {
  return scrollTop >= scrollHeight - viewHeight - tolerance
}

export function shouldAutoScroll(
  current: AutoscrollState,
  nearBottomNow: boolean,
  newEntriesDelta: number,
): AutoscrollState {
  const atBottomNow = current.atBottom || nearBottomNow
  if (atBottomNow) {
    return { atBottom: true, newUpdates: 0 }
  }
  if (newEntriesDelta > 0) {
    return { atBottom: false, newUpdates: current.newUpdates + newEntriesDelta }
  }
  return { atBottom: false, newUpdates: Math.max(0, current.newUpdates) }
}

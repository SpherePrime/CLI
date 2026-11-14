export const workbenchHeightBounds = { min: 140, max: 1200, default: 340 }

export function defaultWorkbenchHeight(viewportHeight = 800) {
  return Math.max(
    workbenchHeightBounds.min,
    Math.min(workbenchHeightBounds.default, Math.round(viewportHeight * 0.42))
  )
}

export function clampWorkbenchHeight(value: number, availableHeight?: number) {
  const upper =
    typeof availableHeight === 'number' && Number.isFinite(availableHeight)
      ? Math.max(
          workbenchHeightBounds.min,
          Math.min(workbenchHeightBounds.max, Math.round(availableHeight))
        )
      : workbenchHeightBounds.max
  if (!Number.isFinite(value)) return Math.min(workbenchHeightBounds.default, upper)
  return Math.max(workbenchHeightBounds.min, Math.min(upper, Math.round(value)))
}

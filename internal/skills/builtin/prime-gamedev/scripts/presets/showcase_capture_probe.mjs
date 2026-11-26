export async function runShowcaseCaptureProbe({
  page,
  helpers,
  angles = ['front', 'right', 'back', 'left', 'quarter'],
  settleMs = 90
}) {
  const elements = await page.evaluate(() => {
    const sc = window.__SHOWCASE__
    if (!sc) return []
    if (typeof sc.listElements === 'function') return sc.listElements()
    if (Array.isArray(sc.elements)) return sc.elements
    return []
  })

  if (!Array.isArray(elements) || elements.length === 0) {
    return {
      ok: false,
      reason: 'window.__SHOWCASE__ has no elements to photograph',
      captures: []
    }
  }

  const captures = []
  for (let i = 0; i < elements.length; i += 1) {
    const item = elements[i]
    const id = typeof item === 'string' ? item : String(item.id || item.name || `element-${i + 1}`)
    const itemAngles = Array.isArray(item?.angles) && item.angles.length > 0 ? item.angles : angles

    for (const angle of itemAngles) {
      const focused = await page.evaluate(
        ({ elementId, viewAngle }) => {
          const sc = window.__SHOWCASE__
          if (!sc || typeof sc.focusElement !== 'function') return false
          sc.focusElement(elementId, viewAngle)
          return true
        },
        { elementId: id, viewAngle: angle }
      )
      if (!focused) {
        return {
          ok: false,
          reason: 'window.__SHOWCASE__.focusElement(id, angle) is not available',
          captures
        }
      }
      await page.waitForTimeout(settleMs)
      const safeName = `${id}-${angle}`.replace(/[^a-zA-Z0-9._-]+/g, '_')
      const filePath = await helpers.captureEventFrame(`showcase-${safeName}`)
      captures.push({ id, angle, filePath })
    }
    helpers.markCheckpoint(`showcase:${id}`, { views: itemAngles.length })
  }

  return {
    ok: captures.length > 0,
    elementCount: elements.length,
    captures
  }
}

export async function runBot({ page, helpers }) {
  await page.waitForTimeout(250)
  return runShowcaseCaptureProbe({ page, helpers })
}

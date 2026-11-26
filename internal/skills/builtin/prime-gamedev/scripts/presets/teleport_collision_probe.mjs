export async function runTeleportCollisionProbe({
  page,
  helpers,
  targets = [],
  probeMs = 280,
  fallThresholdOffset = 3.0
}) {
  const results = []
  const resolvedTargets =
    targets.length > 0
      ? targets
      : await page.evaluate(() => {
          const dbg = window.__GAME_DEBUG__
          if (!dbg) return []
          if (typeof dbg.getProbeTargets === 'function') return dbg.getProbeTargets()
          if (Array.isArray(dbg.probeTargets)) return dbg.probeTargets
          return []
        })

  for (let i = 0; i < resolvedTargets.length; i += 1) {
    const target = resolvedTargets[i]
    const label = String(target.name || `collision-spot-${i + 1}`)
    const teleportOk = await page.evaluate((t) => {
      const dbg = window.__GAME_DEBUG__
      if (!dbg || typeof dbg.teleport !== 'function') return false
      dbg.teleport(t.x ?? 0, t.y ?? 0, t.z ?? 0)
      return true
    }, target)

    if (!teleportOk) {
      results.push({ label, ok: false, reason: 'window.__GAME_DEBUG__.teleport not available' })
      continue
    }

    await page.waitForTimeout(140)
    const before = await helpers.readGameState()
    const startY = Number(before?.y ?? before?.player?.y ?? target.y ?? 0)
    const keys =
      Array.isArray(target.probeKeys) && target.probeKeys.length > 0
        ? target.probeKeys
        : ['KeyW', 'KeyD', 'KeyS', 'KeyA']

    const anomalies = []
    for (const key of keys) {
      await helpers.holdKey(key, probeMs)
      const afterStep = await helpers.readGameState()
      const currentY = Number(afterStep?.y ?? afterStep?.player?.y ?? startY)
      const coords = [
        afterStep?.x ?? afterStep?.player?.x,
        currentY,
        afterStep?.z ?? afterStep?.player?.z
      ].filter((v) => v !== undefined)

      if (coords.some((v) => !Number.isFinite(Number(v)))) {
        anomalies.push({ type: 'nan_coordinate', key, coords })
      }
      if (
        Number.isFinite(startY) &&
        Number.isFinite(currentY) &&
        currentY < startY - fallThresholdOffset
      ) {
        anomalies.push({ type: 'floor_fall_through', key, startY, currentY })
      }
    }

    helpers.markCheckpoint(`collision-probe:${label}`, { anomalies: anomalies.length })
    results.push({
      label,
      ok: anomalies.length === 0,
      anomalies
    })
  }

  return results
}

export async function runBot({ page, helpers }) {
  await helpers.clickCanvas(0.5, 0.5)
  await helpers.tapKey('Enter', 80)
  await page.waitForTimeout(300)
  await runTeleportCollisionProbe({ page, helpers })
}

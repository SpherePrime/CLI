function readPosition(state) {
  if (!state || typeof state !== 'object') return null
  const src = state.player && typeof state.player === 'object' ? state.player : state
  const x = Number(src.x)
  const y = Number(src.y)
  const z = src.z !== undefined ? Number(src.z) : 0
  if (!Number.isFinite(x) || !Number.isFinite(y) || !Number.isFinite(z)) return null
  return { x, y, z }
}

function distance2D(a, b) {
  if (!a || !b) return 0
  return Math.hypot(b.x - a.x, (b.z ?? b.y) - (a.z ?? a.y))
}

export async function runPhysicsBoundsProbe({ page, helpers, moveMs = 450 }) {
  const issues = []

  const startPos = readPosition(await helpers.readGameState())
  await helpers.holdKey('KeyD', moveMs)
  const afterCardinal = readPosition(await helpers.readGameState())
  const cardinalDist = distance2D(startPos, afterCardinal)

  await page.keyboard.down('KeyW')
  await page.keyboard.down('KeyD')
  await page.waitForTimeout(moveMs)
  await page.keyboard.up('KeyD')
  await page.keyboard.up('KeyW')
  const afterDiag = readPosition(await helpers.readGameState())
  const diagDist = distance2D(afterCardinal, afterDiag)

  if (cardinalDist > 0.1 && diagDist > cardinalDist * 1.25) {
    issues.push({
      type: 'unnormalized_diagonal_speed',
      cardinalDist: Number(cardinalDist.toFixed(3)),
      diagDist: Number(diagDist.toFixed(3))
    })
  }

  for (const key of ['KeyA', 'KeyS', 'Space']) {
    await helpers.holdKey(key, 350)
  }

  const finalState = await helpers.readGameState()
  const finalPos = readPosition(finalState)
  if (startPos && !finalPos) {
    issues.push({ type: 'invalid_coordinates_after_movement', finalState })
  }

  await helpers.captureEventFrame('physics-bounds-probe')
  helpers.markCheckpoint('physics-bounds-probe', {
    cardinalDist: Number(cardinalDist.toFixed(3)),
    diagDist: Number(diagDist.toFixed(3)),
    issues
  })

  return {
    ok: issues.length === 0,
    cardinalDist,
    diagDist,
    issues
  }
}

export async function runBot({ page, helpers }) {
  await helpers.clickCanvas(0.5, 0.5)
  await helpers.tapKey('Enter', 80)
  await page.waitForTimeout(300)
  await runPhysicsBoundsProbe({ page, helpers })
}

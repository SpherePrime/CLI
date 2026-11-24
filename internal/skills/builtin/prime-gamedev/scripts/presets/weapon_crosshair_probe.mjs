export async function runWeaponCrosshairProbe({
  page,
  helpers,
  crosshairSelector = '[data-crosshair], .crosshair, #crosshair, .reticle',
  fireKey = null,
  reloadKey = 'KeyR',
  shots = 3
}) {
  const crosshairDom = await page.evaluate((selector) => {
    const el = document.querySelector(selector)
    if (!el) return { found: false }
    const rect = el.getBoundingClientRect()
    const style = getComputedStyle(el)
    const visible =
      style.display !== 'none' &&
      style.visibility !== 'hidden' &&
      Number(style.opacity) > 0.05 &&
      rect.width > 0 &&
      rect.height > 0
    return {
      found: true,
      visible,
      rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
    }
  }, crosshairSelector)

  const stateBefore = await helpers.readGameState()
  const debugCrosshair = Boolean(
    stateBefore?.crosshairVisible ?? stateBefore?.weapon?.hasCrosshair ?? stateBefore?.hud?.crosshair
  )
  const crosshairPresent = crosshairDom.visible || debugCrosshair
  const ammoBefore = Number(stateBefore?.ammo ?? stateBefore?.weapon?.ammo ?? Number.NaN)

  if (crosshairDom.found && crosshairDom.visible) {
    await helpers.captureElement(crosshairSelector, 'weapon-crosshair', { padding: 40 })
  } else {
    await helpers.captureEventFrame('weapon-crosshair-view')
  }

  for (let i = 0; i < shots; i += 1) {
    if (fireKey) {
      await helpers.tapKey(fireKey, 90)
    } else {
      await helpers.clickCanvas(0.5, 0.5)
    }
    await page.waitForTimeout(160)
  }

  const stateAfterFire = await helpers.readGameState()
  const ammoAfterFire = Number(stateAfterFire?.ammo ?? stateAfterFire?.weapon?.ammo ?? Number.NaN)
  await helpers.captureEventFrame('weapon-after-fire')

  await helpers.tapKey(reloadKey, 90)
  await page.waitForTimeout(500)
  const stateAfterReload = await helpers.readGameState()
  const ammoAfterReload = Number(stateAfterReload?.ammo ?? stateAfterReload?.weapon?.ammo ?? Number.NaN)

  const issues = []
  if (!crosshairPresent) {
    issues.push('Crosshair not detected via DOM selector or window.__GAME_DEBUG__')
  }
  if (Number.isFinite(ammoBefore) && Number.isFinite(ammoAfterFire) && ammoAfterFire >= ammoBefore) {
    issues.push(`Ammo did not decrease after ${shots} shots (${ammoBefore} -> ${ammoAfterFire})`)
  }

  helpers.markCheckpoint('weapon-crosshair-probe', {
    crosshairPresent,
    ammoBefore: Number.isFinite(ammoBefore) ? ammoBefore : null,
    ammoAfterFire: Number.isFinite(ammoAfterFire) ? ammoAfterFire : null,
    ammoAfterReload: Number.isFinite(ammoAfterReload) ? ammoAfterReload : null,
    issues
  })

  return {
    ok: issues.length === 0,
    crosshairPresent,
    crosshairDom,
    ammoBefore,
    ammoAfterFire,
    ammoAfterReload,
    issues
  }
}

export async function runBot({ page, helpers }) {
  await helpers.clickCanvas(0.5, 0.5)
  await helpers.tapKey('Enter', 80)
  await page.waitForTimeout(300)
  await runWeaponCrosshairProbe({ page, helpers })
}

export async function runBalanceAndStateProbe({
  page,
  helpers,
  pauseKey = 'Escape',
  minFps = 50,
  restartSelector = '[data-action="restart"], button:has-text("Restart"), button:has-text("Заново")'
}) {
  const issues = []
  const initial = await helpers.readGameState()
  await helpers.captureEventFrame('state-gameplay-active')

  await helpers.holdKey('KeyD', 400)
  await helpers.tapKey('Space', 100)
  const midPlay = await helpers.readGameState()

  await helpers.tapKey(pauseKey, 80)
  await page.waitForTimeout(250)
  const paused = await helpers.readGameState()
  await helpers.captureEventFrame('state-paused')

  await helpers.tapKey(pauseKey, 80)
  await page.waitForTimeout(250)

  const restartButton = page.locator(restartSelector).first()
  if ((await restartButton.count()) > 0 && (await restartButton.isVisible())) {
    await restartButton.click()
    await page.waitForTimeout(350)
  } else {
    await page.evaluate(() => {
      const dbg = window.__GAME_DEBUG__
      if (dbg && typeof dbg.restart === 'function') {
        dbg.restart()
      }
    })
    await page.waitForTimeout(350)
  }

  const afterRestart = await helpers.readGameState()
  await helpers.captureEventFrame('state-after-restart')

  const scoreAfterRestart = Number(afterRestart?.score ?? Number.NaN)
  if (Number.isFinite(scoreAfterRestart) && scoreAfterRestart > 0) {
    issues.push(`Score did not reset cleanly on restart (score=${scoreAfterRestart})`)
  }

  const sampledFps = Number(midPlay?.fps ?? Number.NaN)
  if (Number.isFinite(sampledFps) && sampledFps > 0 && sampledFps < minFps) {
    issues.push(`FPS dropped below target (${sampledFps} < ${minFps})`)
  }

  helpers.recordDifficultySample({
    hpStart: initial?.hp ?? initial?.player?.hp ?? null,
    hpMid: midPlay?.hp ?? midPlay?.player?.hp ?? null,
    enemyCount: midPlay?.enemyCount ?? midPlay?.enemies?.length ?? null,
    fps: midPlay?.fps ?? null,
    pausedStateDetected: Boolean(paused?.paused ?? paused?.state === 'paused')
  })

  helpers.markCheckpoint('balance-and-state-probe', { issues })
  return {
    ok: issues.length === 0,
    initial,
    midPlay,
    paused,
    afterRestart,
    issues
  }
}

export async function runBot({ page, helpers }) {
  await helpers.clickCanvas(0.5, 0.5)
  await helpers.tapKey('Enter', 80)
  await page.waitForTimeout(300)
  await runBalanceAndStateProbe({ page, helpers })
}

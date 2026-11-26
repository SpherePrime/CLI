export function createRuntimeMonitor(page) {
  const pageErrors = []
  const consoleErrors = []
  const consoleWarnings = []
  const failedRequests = []
  const httpErrors = []

  page.on('pageerror', (error) => {
    pageErrors.push({
      time: new Date().toISOString(),
      message: error?.message || String(error),
      stack: error?.stack || ''
    })
  })

  page.on('console', (msg) => {
    const type = msg.type()
    const text = msg.text()
    if (type === 'error') {
      consoleErrors.push({ time: new Date().toISOString(), text })
      return
    }
    if (type === 'warning') {
      const lower = text.toLowerCase()
      if (
        lower.includes('webgl') ||
        lower.includes('shader') ||
        lower.includes('texture') ||
        lower.includes('nan') ||
        lower.includes('collision') ||
        lower.includes('physics')
      ) {
        consoleWarnings.push({ time: new Date().toISOString(), text })
      }
    }
  })

  page.on('requestfailed', (request) => {
    failedRequests.push({
      time: new Date().toISOString(),
      url: request.url(),
      resourceType: request.resourceType(),
      failure: request.failure()?.errorText || 'failed'
    })
  })

  page.on('response', (response) => {
    if (response.status() >= 400) {
      httpErrors.push({
        time: new Date().toISOString(),
        url: response.url(),
        status: response.status(),
        resourceType: response.request().resourceType()
      })
    }
  })

  return {
    snapshot() {
      return {
        pageErrors,
        consoleErrors,
        consoleWarnings,
        failedRequests,
        httpErrors
      }
    }
  }
}

export function createTelemetryTracker({
  floorYLimit,
  stuckThresholdMs = 2500,
  minFpsTarget = 50
} = {}) {
  const samples = []
  const checkpoints = []
  const difficultySamples = []
  const physicsAnomalies = []
  let activeMovementStartedAt = null
  let anchorPosition = null
  let stuckReportedForSpan = false

  function hasNonFiniteValues(obj) {
    if (obj === null || obj === undefined) {
      return false
    }
    if (typeof obj === 'number') {
      return !Number.isFinite(obj)
    }
    if (Array.isArray(obj)) {
      return obj.some((item) => hasNonFiniteValues(item))
    }
    if (typeof obj === 'object') {
      return Object.values(obj).some((val) => hasNonFiniteValues(val))
    }
    return false
  }

  function extractPosition(state) {
    const target = state?.player || state
    if (!target || typeof target !== 'object') {
      return null
    }
    const x = Number(target.x ?? target.posX ?? target.position?.x)
    const y = Number(target.y ?? target.posY ?? target.position?.y)
    const z = Number(target.z ?? target.posZ ?? target.position?.z ?? 0)
    if (!Number.isFinite(x) || !Number.isFinite(y)) {
      return null
    }
    return { x, y, z: Number.isFinite(z) ? z : 0 }
  }

  function recordState(state, { movingInputActive = false, nowMs = Date.now() } = {}) {
    if (!state || typeof state !== 'object') {
      return
    }
    const sample = {
      timeMs: nowMs,
      ...state
    }
    samples.push(sample)

    if (hasNonFiniteValues(state)) {
      physicsAnomalies.push({
        type: 'non_finite_state',
        timeMs: nowMs,
        detail: 'Detected NaN or Infinity in game telemetry',
        sample
      })
    }

    if (state.outOfBounds === true || state.clipped === true || state.fellThroughFloor === true) {
      physicsAnomalies.push({
        type: state.fellThroughFloor ? 'fall_through_floor_or_texture' : 'collision_or_oob',
        timeMs: nowMs,
        detail: 'Game state reported out-of-bounds, clipping, or floor fall-through',
        sample
      })
    }

    const pos = extractPosition(state)
    if (pos) {
      if (Number.isFinite(floorYLimit) && pos.y > floorYLimit) {
        physicsAnomalies.push({
          type: 'exceeded_floor_y_limit',
          timeMs: nowMs,
          detail: `Position y=${pos.y} exceeded floorYLimit=${floorYLimit}`,
          pos
        })
      }

      if (movingInputActive) {
        if (activeMovementStartedAt === null || !anchorPosition) {
          activeMovementStartedAt = nowMs
          anchorPosition = pos
          stuckReportedForSpan = false
        } else {
          const dist = Math.hypot(
            pos.x - anchorPosition.x,
            pos.y - anchorPosition.y,
            pos.z - anchorPosition.z
          )
          if (dist > 2) {
            activeMovementStartedAt = nowMs
            anchorPosition = pos
            stuckReportedForSpan = false
          } else if (!stuckReportedForSpan && nowMs - activeMovementStartedAt >= stuckThresholdMs) {
            stuckReportedForSpan = true
            physicsAnomalies.push({
              type: 'possible_stuck_or_softlock',
              timeMs: nowMs,
              detail: `Player displacement remained <= 2 units for ${nowMs - activeMovementStartedAt}ms while movement input was active`,
              pos
            })
          }
        }
      } else {
        activeMovementStartedAt = null
        anchorPosition = null
        stuckReportedForSpan = false
      }
    }
  }

  function markCheckpoint(name, meta = {}) {
    checkpoints.push({
      time: new Date().toISOString(),
      name,
      meta
    })
  }

  function recordDifficultySample(sample) {
    difficultySamples.push({
      time: new Date().toISOString(),
      ...sample
    })
  }

  function summary() {
    const fpsValues = samples
      .map((s) => Number(s.fps))
      .filter((v) => Number.isFinite(v) && v > 0)
    const minFps = fpsValues.length > 0 ? Math.min(...fpsValues) : null
    const avgFps =
      fpsValues.length > 0
        ? Number((fpsValues.reduce((sum, v) => sum + v, 0) / fpsValues.length).toFixed(1))
        : null
    const belowTargetSamples = fpsValues.filter((v) => v < minFpsTarget).length
    const performanceAnomalies = []
    if (avgFps !== null && avgFps < minFpsTarget) {
      performanceAnomalies.push({
        type: 'low_fps',
        targetFps: minFpsTarget,
        avgFps,
        minFps,
        belowTargetSamples,
        detail: `Average FPS (${avgFps}) dropped below the ${minFpsTarget} FPS target (min=${minFps})`
      })
    }

    return {
      sampleCount: samples.length,
      checkpointCount: checkpoints.length,
      checkpoints,
      difficultySamples,
      physicsAnomalies,
      performanceAnomalies,
      fps: {
        min: minFps,
        avg: avgFps,
        target: minFpsTarget,
        belowTargetSamples
      }
    }
  }

  return {
    recordState,
    markCheckpoint,
    recordDifficultySample,
    summary
  }
}

export async function inspectCanvasHealth(page) {
  return page.evaluate(() => {
    const canvases = Array.from(document.querySelectorAll('canvas'))
    if (canvases.length === 0) {
      return { canvasCount: 0, canvases: [] }
    }
    const results = canvases.map((canvas, idx) => {
      const rect = canvas.getBoundingClientRect()
      const info = {
        index: idx,
        id: canvas.id || '',
        width: canvas.width,
        height: canvas.height,
        cssWidth: Math.round(rect.width),
        cssHeight: Math.round(rect.height),
        visible: rect.width > 0 && rect.height > 0,
        blankOrUniform: null
      }
      if (rect.width > 0 && rect.height > 0 && canvas.width > 0 && canvas.height > 0) {
        try {
          const probe = document.createElement('canvas')
          probe.width = 16
          probe.height = 16
          const ctx = probe.getContext('2d')
          if (ctx) {
            ctx.drawImage(canvas, 0, 0, 16, 16)
            const pixels = ctx.getImageData(0, 0, 16, 16).data
            const firstR = pixels[0]
            const firstG = pixels[1]
            const firstB = pixels[2]
            const firstA = pixels[3]
            let differs = false
            for (let p = 4; p < pixels.length; p += 4) {
              if (
                Math.abs(pixels[p] - firstR) > 2 ||
                Math.abs(pixels[p + 1] - firstG) > 2 ||
                Math.abs(pixels[p + 2] - firstB) > 2 ||
                Math.abs(pixels[p + 3] - firstA) > 2
              ) {
                differs = true
                break
              }
            }
            info.blankOrUniform = !differs
          }
        } catch {
        }
      }
      return info
    })
    return { canvasCount: canvases.length, canvases: results }
  })
}

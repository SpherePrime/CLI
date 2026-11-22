#!/usr/bin/env node
import { existsSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { extractAndSummarizeFrames } from './extract_frames.mjs'
import {
  parseFlagArgs,
  parsePositiveInteger,
  parsePositiveNumber,
  resolveOutputDirectory
} from './lib/args.mjs'
import { launchGameBrowser, loadPlaywright } from './lib/playwright_loader.mjs'
import {
  createRuntimeMonitor,
  createTelemetryTracker,
  inspectCanvasHealth
} from './lib/telemetry.mjs'

const USAGE = `Usage:
  node web_bot_runner.mjs --url <game-url> [options]

Options:
  --url <url>                  Target game URL (required)
  --bot <path>                 Optional JS module exporting runBot({ page, canvas, helpers })
  --out <dir>                  Output directory (defaults to a fresh system temp folder)
  --width <int>                Viewport and video width (default: 1280)
  --height <int>               Viewport and video height (default: 720)
  --fps <number>               Frames per second extracted from video (default: 4)
  --max-frames <int>           Max frames to retain (default: 120)
  --poll-ms <int>              Telemetry polling interval in ms (default: 150)
  --capture-interval-ms <int>  Fallback PNG capture interval in ms (default: 350)
  --explore-ms <int>           Built-in exploratory bot duration when --bot is omitted (default: 10000)
  --floor-y-limit <number>     Optional max Y coordinate threshold for fall-through checks
  --state-expr <js>            JS expression evaluated in page to read game state (default: window.__GAME_DEBUG__)
  --playwright-path <path>     Optional explicit path to playwright package
  --headed                     Run Chromium with visible window`

async function runDefaultExploratoryBot({ page, canvas, helpers, durationMs }) {
  helpers.markCheckpoint('exploratory_start')
  const box = canvas ? await canvas.boundingBox() : null
  if (box) {
    await helpers.clickCanvas(0.5, 0.5)
    await page.waitForTimeout(250)
  }

  const probeKeys = [
    ['ArrowRight', 900],
    ['ArrowLeft', 900],
    ['Space', 250],
    ['KeyD', 900],
    ['KeyW', 450],
    ['KeyA', 900],
    ['KeyS', 450],
    ['ArrowUp', 450],
    ['ArrowDown', 450],
    ['Enter', 200]
  ]

  const deadline = Date.now() + durationMs
  let step = 0
  while (Date.now() < deadline) {
    const [key, holdMs] = probeKeys[step % probeKeys.length]
    await helpers.holdKey(key, Math.min(holdMs, Math.max(100, deadline - Date.now())))
    if (step % 4 === 3 && box) {
      await helpers.clickCanvas(0.25 + ((step * 0.17) % 0.5), 0.5)
    }
    step += 1
  }
  helpers.markCheckpoint('exploratory_end', { stepsExecuted: step })
}

async function main() {
  const { flags } = parseFlagArgs(process.argv.slice(2))
  if (flags.help || typeof flags.url !== 'string' || !flags.url.trim()) {
    console.log(USAGE)
    process.exit(flags.help ? 0 : 1)
  }

  const targetUrl = flags.url.trim()
  const outDir = resolveOutputDirectory(flags.out, 'prime-gamedev-run')
  const videoDir = join(outDir, 'video')
  const fallbackFramesDir = join(outDir, 'live-captures')
  const eventFramesDir = join(outDir, 'events')
  mkdirSync(videoDir, { recursive: true })
  mkdirSync(fallbackFramesDir, { recursive: true })
  mkdirSync(eventFramesDir, { recursive: true })

  const width = parsePositiveInteger(flags.width, 1280)
  const height = parsePositiveInteger(flags.height, 720)
  const fps = parsePositiveNumber(flags.fps, 4)
  const maxFrames = parsePositiveInteger(flags['max-frames'], 120)
  const pollMs = parsePositiveInteger(flags['poll-ms'], 150)
  const captureIntervalMs = parsePositiveInteger(flags['capture-interval-ms'], 350)
  const exploreMs = parsePositiveInteger(flags['explore-ms'], 10000)
  const floorYLimit =
    flags['floor-y-limit'] !== undefined ? Number(flags['floor-y-limit']) : Number.NaN
  const stateExpr =
    typeof flags['state-expr'] === 'string' && flags['state-expr'].trim()
      ? flags['state-expr'].trim()
      : 'window.__GAME_DEBUG__'

  const playwright = await loadPlaywright(
    typeof flags['playwright-path'] === 'string' ? flags['playwright-path'] : undefined
  )
  const browser = await launchGameBrowser(playwright.chromium, {
    headed: Boolean(flags.headed)
  })

  const context = await browser.newContext({
    viewport: { width, height },
    recordVideo: {
      dir: videoDir,
      size: { width, height }
    }
  })

  const page = await context.newPage()
  const video = page.video()
  const runtimeMonitor = createRuntimeMonitor(page)
  const telemetry = createTelemetryTracker({ floorYLimit })
  const eventCaptures = []
  let activeMovementKeys = 0
  let pollingActive = true
  let fallbackIndex = 0
  let botError = null
  let canvasHealthStart = null
  let canvasHealthEnd = null

  async function readGameState() {
    try {
      return await page.evaluate((expr) => {
        try {
          const fn = new Function(`return (${expr})`)
          const value = fn()
          if (typeof value === 'function') {
            return value()
          }
          return value ?? null
        } catch {
          return null
        }
      }, stateExpr)
    } catch {
      return null
    }
  }

  async function captureEventFrame(label) {
    const safeLabel = String(label || 'event').replace(/[^a-zA-Z0-9_-]+/g, '-')
    const fileName = `${String(eventCaptures.length + 1).padStart(3, '0')}-${safeLabel}.png`
    const filePath = join(eventFramesDir, fileName)
    await page.screenshot({ path: filePath })
    eventCaptures.push({ label: safeLabel, path: filePath, time: new Date().toISOString() })
    return filePath
  }

  let pollTimer = null
  let captureTimer = null

  try {
    await page.goto(targetUrl, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForTimeout(800)

    const canvas = page.locator('canvas').first()
    const canvasVisible = (await page.locator('canvas').count()) > 0
    canvasHealthStart = await inspectCanvasHealth(page)

    const helpers = {
      async holdKey(key, durationMs = 400) {
        const isMove = /^(Arrow|Key[WASD]$)/i.test(key)
        if (isMove) {
          activeMovementKeys += 1
        }
        await page.keyboard.down(key)
        try {
          await page.waitForTimeout(Math.max(20, durationMs))
        } finally {
          await page.keyboard.up(key)
          if (isMove) {
            activeMovementKeys = Math.max(0, activeMovementKeys - 1)
          }
        }
      },
      async tapKey(key, holdMs = 80) {
        await page.keyboard.down(key)
        try {
          await page.waitForTimeout(Math.max(20, holdMs))
        } finally {
          await page.keyboard.up(key)
        }
      },
      async clickCanvas(relX = 0.5, relY = 0.5) {
        const box = canvasVisible ? await canvas.boundingBox() : null
        if (!box) {
          await page.mouse.click(Math.round(width * relX), Math.round(height * relY))
          return
        }
        await page.mouse.click(box.x + box.width * relX, box.y + box.height * relY)
      },
      async moveMouseCanvas(relX = 0.5, relY = 0.5) {
        const box = canvasVisible ? await canvas.boundingBox() : null
        if (!box) {
          await page.mouse.move(Math.round(width * relX), Math.round(height * relY))
          return
        }
        await page.mouse.move(box.x + box.width * relX, box.y + box.height * relY)
      },
      markCheckpoint(name, meta) {
        telemetry.markCheckpoint(name, meta)
      },
      captureEventFrame,
      readGameState,
      recordDifficultySample(sample) {
        telemetry.recordDifficultySample(sample)
      }
    }

    pollTimer = setInterval(async () => {
      if (!pollingActive) {
        return
      }
      const state = await readGameState()
      if (state) {
        telemetry.recordState(state, {
          movingInputActive: activeMovementKeys > 0
        })
      }
    }, pollMs)

    captureTimer = setInterval(async () => {
      if (!pollingActive) {
        return
      }
      fallbackIndex += 1
      const name = `capture_${String(fallbackIndex).padStart(5, '0')}.png`
      try {
        await page.screenshot({ path: join(fallbackFramesDir, name) })
      } catch {
      }
    }, captureIntervalMs)

    await captureEventFrame('initial-state')

    if (typeof flags.bot === 'string' && flags.bot.trim()) {
      const botPath = resolve(flags.bot.trim())
      if (!existsSync(botPath)) {
        throw new Error(`Bot script not found: ${botPath}`)
      }
      const botModule = await import(pathToFileURL(botPath).href)
      const runner = botModule.runBot || botModule.default
      if (typeof runner !== 'function') {
        throw new Error(`Bot module ${botPath} must export runBot({ page, canvas, helpers })`)
      }
      await runner({
        page,
        canvas: canvasVisible ? canvas : null,
        helpers
      })
    } else {
      await runDefaultExploratoryBot({
        page,
        canvas: canvasVisible ? canvas : null,
        helpers,
        durationMs: exploreMs
      })
    }

    await captureEventFrame('final-state')
    canvasHealthEnd = await inspectCanvasHealth(page)
  } catch (error) {
    botError = error?.stack || error?.message || String(error)
  } finally {
    pollingActive = false
    if (pollTimer) {
      clearInterval(pollTimer)
    }
    if (captureTimer) {
      clearInterval(captureTimer)
    }
    await context.close()
    await browser.close()
  }

  const videoPath = video ? await video.path() : null
  let framesSummary = null
  let extractionError = null

  try {
    if (videoPath && existsSync(videoPath)) {
      framesSummary = extractAndSummarizeFrames({
        video: videoPath,
        out: join(outDir, 'frames_output'),
        fps,
        maxFrames
      })
      rmSync(fallbackFramesDir, { recursive: true, force: true })
    }
  } catch (error) {
    extractionError = error?.message || String(error)
  }

  if (!framesSummary) {
    try {
      framesSummary = extractAndSummarizeFrames({
        framesDir: fallbackFramesDir,
        out: join(outDir, 'frames_output'),
        fps: 1000 / captureIntervalMs,
        maxFrames
      })
    } catch (fallbackError) {
      extractionError = extractionError || fallbackError?.message || String(fallbackError)
    }
  }

  const runtime = runtimeMonitor.snapshot()
  const telemetrySummary = telemetry.summary()
  const report = {
    url: targetUrl,
    outDir,
    videoPath,
    botError,
    extractionError,
    canvasHealthStart,
    canvasHealthEnd,
    runtime,
    telemetry: telemetrySummary,
    eventCaptures,
    frames: framesSummary
  }

  const reportPath = join(outDir, 'report.json')
  writeFileSync(reportPath, JSON.stringify(report, null, 2))

  console.log(`GameDev bot run output: ${outDir}`)
  console.log(`Report: ${reportPath}`)
  if (framesSummary) {
    console.log(
      `Frames: ${framesSummary.frameCount}, Contact sheets: ${framesSummary.contactSheetCount} (${framesSummary.sheetsDir})`
    )
  }
  console.log(
    `Runtime issues: pageErrors=${runtime.pageErrors.length}, consoleErrors=${runtime.consoleErrors.length}, httpErrors=${runtime.httpErrors.length}, failedRequests=${runtime.failedRequests.length}, physicsAnomalies=${telemetrySummary.physicsAnomalies.length}`
  )

  if (botError || runtime.pageErrors.length > 0 || telemetrySummary.physicsAnomalies.length > 0) {
    process.exit(1)
  }
}

main().catch((error) => {
  console.error(error?.stack || error?.message || error)
  process.exit(2)
})

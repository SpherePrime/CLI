#!/usr/bin/env node
import { spawn, spawnSync } from 'node:child_process'
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { parseFlagArgs, resolveOutputDirectory } from './lib/args.mjs'
import { resolveFfmpegBinaries } from './lib/ffmpeg.mjs'
import { buildWindowFfmpegInputArgs, waitForTargetWindow } from './lib/window_target.mjs'
import { extractAndSummarizeFrames } from './extract_frames.mjs'
import { cropImageWithFfmpeg, parseCropSpecs } from './element_capture.mjs'

function waitMs(ms) {
  return new Promise((resolvePromise) => setTimeout(resolvePromise, ms))
}

function runShellCommand(command, cwd, envExtra = {}, timeoutMs = 45000) {
  return new Promise((resolvePromise) => {
    const child = spawn(command, {
      cwd,
      shell: true,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env, ...envExtra }
    })
    let stdout = ''
    let stderr = ''
    let settled = false
    let timedOut = false

    const finish = (result) => {
      if (settled) {
        return
      }
      settled = true
      clearTimeout(timer)
      resolvePromise(result)
    }

    const timer = setTimeout(async () => {
      timedOut = true
      stderr += `\nBot command timed out after ${timeoutMs}ms and was terminated.`
      await stopChildCleanly(child)
      finish({ code: 124, signal: 'SIGKILL', timedOut: true, stdout, stderr })
    }, Math.max(1000, timeoutMs))

    child.stdout?.on('data', (chunk) => {
      stdout += chunk.toString()
    })
    child.stderr?.on('data', (chunk) => {
      stderr += chunk.toString()
    })
    child.on('exit', (code, signal) => {
      finish({ code, signal, timedOut, stdout, stderr })
    })
    child.on('error', (error) => {
      finish({ code: -1, signal: null, timedOut, stdout, stderr: `${stderr}\n${error.message}` })
    })
  })
}

async function stopChildCleanly(child) {
  if (!child || child.exitCode !== null) {
    return
  }
  if (process.platform === 'win32' && child.pid) {
    spawnSync('taskkill', ['/PID', String(child.pid), '/T', '/F'], {
      stdio: 'ignore',
      windowsHide: true
    })
    return
  }
  await new Promise((resolvePromise) => {
    const timer = setTimeout(() => {
      try {
        child.kill('SIGKILL')
      } catch {
      }
      resolvePromise()
    }, 3000)
    child.once('exit', () => {
      clearTimeout(timer)
      resolvePromise()
    })
    try {
      child.kill('SIGTERM')
    } catch {
      clearTimeout(timer)
      resolvePromise()
    }
  })
}

async function stopFfmpegRecording(recorder) {
  if (!recorder || recorder.exitCode !== null) {
    return
  }
  await new Promise((resolvePromise) => {
    const timer = setTimeout(() => {
      try {
        recorder.kill('SIGTERM')
      } catch {
      }
      resolvePromise()
    }, 5000)
    recorder.once('exit', () => {
      clearTimeout(timer)
      resolvePromise()
    })
    try {
      recorder.stdin.write('q\n')
    } catch {
      clearTimeout(timer)
      try {
        recorder.kill('SIGTERM')
      } catch {
      }
      resolvePromise()
    }
  })
}

function printUsage() {
  console.log(`Usage:
  node native_bot_runner.mjs (--cmd "<game launch command>" | --window "<window title substring>") [--cwd <dir>] [--grab-mode window|title|region] [--window-timeout-ms 15000] [--bot-cmd "<bot command>"] [--bot-timeout-ms 45000] [--duration 15] [--fps 4] [--max-frames 120] [--element-crops "player:100,100,200,200"] [--out <dir>]`)
}

async function main() {
  const { flags } = parseFlagArgs(process.argv.slice(2))
  const windowQuery = typeof flags.window === 'string' ? flags.window.trim() : ''
  const hasCmd = typeof flags.cmd === 'string' && flags.cmd.trim().length > 0
  if (flags.help || (!hasCmd && !windowQuery)) {
    printUsage()
    process.exit(flags.help ? 0 : 1)
  }

  const { ffmpegPath } = resolveFfmpegBinaries()
  if (!ffmpegPath) {
    throw new Error('ffmpeg is required for continuous native gameplay video recording')
  }

  const cwd = resolve(flags.cwd || process.cwd())
  const outDir = resolveOutputDirectory(flags.out, 'prime-gamedev-native')
  const framesOutDir = join(outDir, 'frames_output')
  const elementsDir = join(outDir, 'elements')
  mkdirSync(outDir, { recursive: true })
  mkdirSync(elementsDir, { recursive: true })

  const videoPath = join(outDir, 'playthrough.mp4')
  const startupMs = Math.max(200, Number(flags['startup-ms'] ?? flags.startupMs) || 800)
  const windowTimeoutMs = Math.max(1000, Number(flags['window-timeout-ms'] ?? flags.windowTimeoutMs) || 15000)
  const botTimeoutMs = Math.max(2000, Number(flags['bot-timeout-ms'] ?? flags.botTimeoutMs) || 45000)
  const durationSec = Math.max(3, Number(flags.duration) || 15)
  const fps = Math.max(1, Number(flags.fps) || 4)
  const maxFrames = Math.max(1, Number(flags['max-frames'] ?? flags.maxFrames) || 120)
  const grabMode =
    flags['grab-mode'] === 'title'
      ? 'title'
      : flags['grab-mode'] === 'region'
        ? 'region'
        : 'window'

  let gameProc = null
  let gameStdout = ''
  let gameStderr = ''
  if (hasCmd) {
    gameProc = spawn(flags.cmd, {
      cwd,
      shell: true,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: {
        ...process.env,
        PRIME_GAMEDEV_OUT: outDir,
        PRIME_GAMEDEV_ELEMENTS: elementsDir
      }
    })
    gameProc.stdout?.on('data', (chunk) => {
      gameStdout += chunk.toString()
    })
    gameProc.stderr?.on('data', (chunk) => {
      gameStderr += chunk.toString()
    })
    await waitMs(startupMs)
  }

  const windowTarget = await waitForTargetWindow({
    pid: gameProc?.pid,
    titleQuery: windowQuery,
    timeoutMs: windowTimeoutMs,
    focus: true
  })

  if (!windowTarget) {
    await stopChildCleanly(gameProc)
    throw new Error(
      `Could not find the target game window (pid=${gameProc?.pid || '<none>'}, window=${windowQuery || '<auto>'}). Refusing to record the full computer desktop; ensure the game opens a visible window or pass --window "<title>".`
    )
  }

  const captureSpec = buildWindowFfmpegInputArgs({
    windowTarget,
    recordFps: Number(flags['record-fps'] ?? flags.recordFps) || 30,
    grabMode,
    screen: typeof flags.screen === 'string' ? flags.screen : '1:none'
  })

  const recorderArgs = [
    '-y',
    '-hide_banner',
    '-loglevel',
    'error',
    ...captureSpec.inputArgs,
    ...captureSpec.filterArgs,
    '-pix_fmt',
    'yuv420p',
    '-movflags',
    '+faststart',
    videoPath
  ]
  const recorderProc = spawn(ffmpegPath, recorderArgs, {
    stdio: ['pipe', 'pipe', 'pipe']
  })
  let recorderStderr = ''
  recorderProc.stderr?.on('data', (chunk) => {
    recorderStderr += chunk.toString()
  })

  let botResult = null
  let gameExitedEarly = false
  try {
    if (flags['bot-cmd'] ?? flags.botCmd) {
      const botCmd = flags['bot-cmd'] ?? flags.botCmd
      botResult = await runShellCommand(
        botCmd,
        cwd,
        {
          PRIME_GAMEDEV_OUT: outDir,
          PRIME_GAMEDEV_ELEMENTS: elementsDir,
          PRIME_GAMEDEV_WINDOW_TITLE: windowTarget.title || '',
          PRIME_GAMEDEV_WINDOW_PID: String(windowTarget.pid || gameProc?.pid || ''),
          PRIME_GAMEDEV_WINDOW_X: String(windowTarget.region.x),
          PRIME_GAMEDEV_WINDOW_Y: String(windowTarget.region.y),
          PRIME_GAMEDEV_WINDOW_W: String(windowTarget.region.width),
          PRIME_GAMEDEV_WINDOW_H: String(windowTarget.region.height)
        },
        botTimeoutMs
      )
    } else {
      await waitMs(durationSec * 1000)
    }
    if (gameProc && gameProc.exitCode !== null) {
      gameExitedEarly = true
    }
  } finally {
    await stopFfmpegRecording(recorderProc)
    await stopChildCleanly(gameProc)
  }

  if (!existsSync(videoPath)) {
    throw new Error(`Recorded video file was not created (${videoPath}). ffmpeg stderr: ${recorderStderr}`)
  }

  const frameSummary = extractAndSummarizeFrames({
    video: videoPath,
    out: framesOutDir,
    fps,
    maxFrames,
    cols: Number(flags.cols) || 4,
    rows: Number(flags.rows) || 4
  })

  let elementCaptures = []
  const crops = parseCropSpecs(flags['element-crops'] ?? flags.elementCrops)
  if (crops.length > 0 && frameSummary.frames.length > 0) {
    const sampleIndex = Math.min(frameSummary.frames.length - 1, Math.floor(frameSummary.frames.length / 2))
    elementCaptures = cropImageWithFfmpeg({
      imagePath: frameSummary.frames[sampleIndex].path,
      crops,
      outDir: elementsDir,
      scale: Number(flags['element-scale'] ?? flags.elementScale) || 2
    })
  }

  const report = {
    outDir,
    cwd,
    command: flags.cmd,
    botCommand: (flags['bot-cmd'] ?? flags.botCmd) || null,
    captureMode: captureSpec.mode,
    capturedWindow: {
      pid: windowTarget.pid || null,
      hwnd: windowTarget.hwnd || null,
      title: windowTarget.title || '',
      region: windowTarget.region
    },
    videoPath,
    frames: frameSummary,
    elementCaptures,
    botResult: botResult
      ? {
          code: botResult.code,
          signal: botResult.signal,
          timedOut: Boolean(botResult.timedOut),
          stdout: botResult.stdout.slice(-4000),
          stderr: botResult.stderr.slice(-4000)
        }
      : null,
    gameProcess: gameProc
      ? {
          exitCode: gameProc.exitCode,
          exitedEarly: gameExitedEarly,
          stdout: gameStdout.slice(-4000),
          stderr: gameStderr.slice(-4000)
        }
      : null,
    recorderStderr: recorderStderr.slice(-2000)
  }

  const staticAnomaly = (frameSummary.anomalies || []).find(
    (item) => item.type === 'static_screen' || item.type === 'identical_frame_sizes'
  )

  const reportPath = join(outDir, 'report.json')
  writeFileSync(reportPath, JSON.stringify(report, null, 2))
  console.log(`Native GameDev bot run output: ${outDir}`)
  console.log(`Captured window: "${windowTarget.title}" (${windowTarget.region.width}x${windowTarget.region.height} at ${windowTarget.region.x},${windowTarget.region.y}, mode=${captureSpec.mode})`)
  console.log(`Report: ${reportPath}`)
  console.log(`Video: ${videoPath}`)
  console.log(`Frames: ${frameSummary.frameCount}, Contact sheets: ${frameSummary.contactSheetCount} (${join(framesOutDir, 'sheets')})`)
  if (elementCaptures.length > 0) {
    console.log(`Element close-ups: ${elementCaptures.length} (${elementsDir})`)
  }
  if (staticAnomaly) {
    console.error(`Liveness check FAILED: ${staticAnomaly.detail}`)
  }
  if (gameExitedEarly) {
    console.error(`Game process exited unexpectedly early (code=${gameProc?.exitCode}).`)
  }
  if (botResult?.timedOut) {
    console.error(`Bot command timed out after ${botTimeoutMs}ms and was stopped.`)
  }
  if ((botResult && botResult.code !== 0) || staticAnomaly || gameExitedEarly) {
    process.exit(1)
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    console.error(error?.stack || error?.message || error)
    process.exit(1)
  })
}

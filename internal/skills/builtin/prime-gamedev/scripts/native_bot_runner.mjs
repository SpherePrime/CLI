#!/usr/bin/env node
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { parseFlagArgs, resolveOutputDirectory } from './lib/args.mjs'
import { resolveFfmpegBinaries } from './lib/ffmpeg.mjs'
import { extractAndSummarizeFrames } from './extract_frames.mjs'
import { cropImageWithFfmpeg, parseCropSpecs } from './element_capture.mjs'

function buildCaptureInputArgs(flags) {
  if (process.platform === 'win32') {
    const input = flags.window ? `title=${flags.window}` : 'desktop'
    return ['-f', 'gdigrab', '-framerate', String(Number(flags.recordFps) || 30), '-i', input]
  }
  if (process.platform === 'darwin') {
    const screen = flags.screen || '1:none'
    return ['-f', 'avfoundation', '-framerate', String(Number(flags.recordFps) || 30), '-i', screen]
  }
  const display = process.env.DISPLAY || ':0.0'
  return ['-f', 'x11grab', '-framerate', String(Number(flags.recordFps) || 30), '-i', display]
}

function waitMs(ms) {
  return new Promise((resolvePromise) => setTimeout(resolvePromise, ms))
}

function runShellCommand(command, cwd, envExtra = {}) {
  return new Promise((resolvePromise) => {
    const child = spawn(command, {
      cwd,
      shell: true,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env, ...envExtra }
    })
    let stdout = ''
    let stderr = ''
    child.stdout?.on('data', (chunk) => {
      stdout += chunk.toString()
    })
    child.stderr?.on('data', (chunk) => {
      stderr += chunk.toString()
    })
    child.on('exit', (code, signal) => {
      resolvePromise({ code, signal, stdout, stderr })
    })
    child.on('error', (error) => {
      resolvePromise({ code: -1, signal: null, stdout, stderr: `${stderr}\n${error.message}` })
    })
  })
}

async function stopChildCleanly(child) {
  if (!child || child.exitCode !== null) {
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
  node native_bot_runner.mjs --cmd "<game launch command>" [--cwd <dir>] [--window "<window title>"] [--bot-cmd "<bot command>"] [--duration 15] [--fps 4] [--max-frames 120] [--element-crops "player:100,100,200,200"] [--out <dir>]`)
}

async function main() {
  const { flags } = parseFlagArgs(process.argv.slice(2))
  if (flags.help || !flags.cmd) {
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
  const startupMs = Math.max(200, Number(flags.startupMs) || 1800)
  const durationSec = Math.max(3, Number(flags.duration) || 15)
  const fps = Math.max(1, Number(flags.fps) || 4)
  const maxFrames = Math.max(1, Number(flags.maxFrames) || 120)

  const gameProc = spawn(flags.cmd, {
    cwd,
    shell: true,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: {
      ...process.env,
      PRIME_GAMEDEV_OUT: outDir,
      PRIME_GAMEDEV_ELEMENTS: elementsDir
    }
  })

  let gameStdout = ''
  let gameStderr = ''
  gameProc.stdout?.on('data', (chunk) => {
    gameStdout += chunk.toString()
  })
  gameProc.stderr?.on('data', (chunk) => {
    gameStderr += chunk.toString()
  })

  await waitMs(startupMs)

  const recorderArgs = [
    '-y',
    '-hide_banner',
    '-loglevel',
    'error',
    ...buildCaptureInputArgs(flags),
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
  try {
    if (flags.botCmd) {
      botResult = await runShellCommand(flags.botCmd, cwd, {
        PRIME_GAMEDEV_OUT: outDir,
        PRIME_GAMEDEV_ELEMENTS: elementsDir
      })
    } else {
      await waitMs(durationSec * 1000)
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
  const crops = parseCropSpecs(flags.elementCrops)
  if (crops.length > 0 && frameSummary.frames.length > 0) {
    const sampleIndex = Math.min(frameSummary.frames.length - 1, Math.floor(frameSummary.frames.length / 2))
    elementCaptures = cropImageWithFfmpeg({
      imagePath: frameSummary.frames[sampleIndex].path,
      crops,
      outDir: elementsDir,
      scale: Number(flags.elementScale) || 2
    })
  }

  const report = {
    outDir,
    cwd,
    command: flags.cmd,
    botCommand: flags.botCmd || null,
    videoPath,
    frames: frameSummary,
    elementCaptures,
    botResult: botResult
      ? {
          code: botResult.code,
          signal: botResult.signal,
          stdout: botResult.stdout.slice(-4000),
          stderr: botResult.stderr.slice(-4000)
        }
      : null,
    gameProcess: {
      exitCode: gameProc.exitCode,
      stdout: gameStdout.slice(-4000),
      stderr: gameStderr.slice(-4000)
    },
    recorderStderr: recorderStderr.slice(-2000)
  }

  const reportPath = join(outDir, 'report.json')
  writeFileSync(reportPath, JSON.stringify(report, null, 2))
  console.log(`Native GameDev bot run output: ${outDir}`)
  console.log(`Report: ${reportPath}`)
  console.log(`Video: ${videoPath}`)
  console.log(`Frames: ${frameSummary.frameCount}, Contact sheets: ${frameSummary.contactSheetCount} (${join(framesOutDir, 'sheets')})`)
  if (elementCaptures.length > 0) {
    console.log(`Element close-ups: ${elementCaptures.length} (${elementsDir})`)
  }
  if (botResult && botResult.code !== 0) {
    process.exit(1)
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    console.error(error?.stack || error?.message || error)
    process.exit(1)
  })
}

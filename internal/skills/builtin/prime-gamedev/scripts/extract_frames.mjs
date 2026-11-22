#!/usr/bin/env node
import { existsSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  parseFlagArgs,
  parsePositiveInteger,
  parsePositiveNumber,
  resolveOutputDirectory
} from './lib/args.mjs'
import {
  buildContactSheets,
  detectVideoAnomalies,
  extractVideoToFrames,
  ingestFrameDirectory,
  probeVideoMetadata,
  resolveFfmpegBinaries
} from './lib/ffmpeg.mjs'

const USAGE = `Usage:
  node extract_frames.mjs --video <path> [options]
  node extract_frames.mjs --frames-dir <dir> [options]

Options:
  --video <path>        Path to recorded gameplay video (.webm, .mp4, .mkv, ...)
  --frames-dir <dir>    Directory of existing PNG/JPG/WebP frames (alternative to --video)
  --out <dir>           Output directory (defaults to a fresh system temp folder)
  --fps <number>        Frames per second to extract from video (default: 4)
  --max-frames <int>    Maximum frames to keep via even sampling (default: 120)
  --frame-width <int>   Target width for extracted frames, 0 keeps native (default: 960)
  --cols <int>          Contact sheet columns (default: 4)
  --rows <int>          Contact sheet rows (default: 4)
  --tile-width <int>    Width of each tile inside contact sheets (default: 360)`

export function extractAndSummarizeFrames(options) {
  const fps = parsePositiveNumber(options.fps, 4)
  const maxFrames = parsePositiveInteger(options.maxFrames, 120)
  const frameWidth =
    options.frameWidth === 0 || options.frameWidth === '0'
      ? 0
      : parsePositiveInteger(options.frameWidth, 960)
  const cols = parsePositiveInteger(options.cols, 4)
  const rows = parsePositiveInteger(options.rows, 4)
  const tileWidth = parsePositiveInteger(options.tileWidth, 360)

  const outDir = resolveOutputDirectory(options.out, 'prime-gamedev-frames')
  const framesDir = join(outDir, 'frames')
  const sheetsDir = join(outDir, 'sheets')
  mkdirSync(outDir, { recursive: true })

  const { ffmpegPath, ffprobePath } = resolveFfmpegBinaries()
  let sourceType = 'frames-dir'
  let videoMeta = null
  let frames = []

  if (options.video) {
    const videoPath = resolve(options.video)
    if (!existsSync(videoPath)) {
      throw new Error(`Video file not found: ${videoPath}`)
    }
    if (!ffmpegPath) {
      throw new Error(
        'ffmpeg binary was not found on PATH or standard install directories. Install ffmpeg or provide --frames-dir.'
      )
    }
    sourceType = 'video'
    videoMeta = probeVideoMetadata(ffprobePath, videoPath)
    frames = extractVideoToFrames({
      ffmpegPath,
      videoPath,
      framesDir,
      fps,
      maxFrames,
      frameWidth
    })
    rmSync(join(framesDir, '_raw'), { recursive: true, force: true })
  } else if (options.framesDir) {
    const sourceDir = resolve(options.framesDir)
    if (!existsSync(sourceDir)) {
      throw new Error(`Frames directory not found: ${sourceDir}`)
    }
    frames = ingestFrameDirectory({
      sourceDir,
      framesDir,
      maxFrames,
      assumedFps: fps
    })
  } else {
    throw new Error('Either --video or --frames-dir is required.')
  }

  const contactSheets = buildContactSheets({
    ffmpegPath,
    frames,
    sheetsDir,
    cols,
    rows,
    tileWidth
  })

  const anomalies = detectVideoAnomalies({
    ffmpegPath,
    videoPath: options.video ? resolve(options.video) : null,
    frames
  })

  const manifest = {
    sourceType,
    videoPath: options.video ? resolve(options.video) : null,
    videoMeta,
    outDir,
    framesDir,
    sheetsDir,
    ffmpegAvailable: Boolean(ffmpegPath),
    fps,
    maxFrames,
    frameCount: frames.length,
    contactSheetCount: contactSheets.length,
    contactSheets,
    frames,
    anomalies
  }

  const manifestPath = join(outDir, 'manifest.json')
  writeFileSync(manifestPath, JSON.stringify(manifest, null, 2))
  return { ...manifest, manifestPath }
}

function main() {
  const { flags } = parseFlagArgs(process.argv.slice(2))
  if (flags.help || (!flags.video && !flags['frames-dir'])) {
    console.log(USAGE)
    process.exit(flags.help ? 0 : 1)
  }

  const result = extractAndSummarizeFrames({
    video: typeof flags.video === 'string' ? flags.video : undefined,
    framesDir: typeof flags['frames-dir'] === 'string' ? flags['frames-dir'] : undefined,
    out: typeof flags.out === 'string' ? flags.out : undefined,
    fps: flags.fps,
    maxFrames: flags['max-frames'],
    frameWidth: flags['frame-width'],
    cols: flags.cols,
    rows: flags.rows,
    tileWidth: flags['tile-width']
  })

  console.log(
    `Extracted ${result.frameCount} frame(s) and ${result.contactSheetCount} contact sheet(s) -> ${result.outDir}`
  )
  console.log(`Manifest: ${result.manifestPath}`)
  if (result.anomalies.length > 0) {
    console.log(`Anomalies detected: ${result.anomalies.length}`)
  }
}

const isEntrypoint =
  process.argv[1] && resolve(process.argv[1]) === resolve(fileURLToPath(import.meta.url))

if (isEntrypoint) {
  try {
    main()
  } catch (error) {
    console.error(error?.message || error)
    process.exit(2)
  }
}

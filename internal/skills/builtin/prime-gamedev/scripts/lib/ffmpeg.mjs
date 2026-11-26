import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  statSync
} from 'node:fs'
import { extname, join, resolve } from 'node:path'

const IMAGE_EXTENSIONS = new Set(['.png', '.jpg', '.jpeg', '.webp'])

function probeBinary(candidate) {
  const result = spawnSync(candidate, ['-version'], {
    encoding: 'utf8',
    windowsHide: true
  })
  return !result.error && result.status === 0
}

function findWingetBinaries(binaryBaseName) {
  const localAppData = process.env.LOCALAPPDATA
  if (!localAppData) {
    return []
  }
  const packagesDir = join(localAppData, 'Microsoft', 'WinGet', 'Packages')
  if (!existsSync(packagesDir)) {
    return []
  }
  const matches = []
  let entries = []
  try {
    entries = readdirSync(packagesDir, { withFileTypes: true })
  } catch {
    return matches
  }
  for (const entry of entries) {
    if (!entry.isDirectory() || !entry.name.toLowerCase().includes('ffmpeg')) {
      continue
    }
    const root = join(packagesDir, entry.name)
    let subEntries = []
    try {
      subEntries = readdirSync(root, { withFileTypes: true })
    } catch {
      continue
    }
    for (const sub of subEntries) {
      if (!sub.isDirectory()) {
        continue
      }
      const candidate = join(root, sub.name, 'bin', `${binaryBaseName}.exe`)
      if (existsSync(candidate)) {
        matches.push(candidate)
      }
    }
  }
  return matches
}

export function resolveFfmpegBinaries() {
  const isWin = process.platform === 'win32'
  const exeSuffix = isWin ? '.exe' : ''
  const ffmpegCandidates = [
    process.env.FFMPEG_PATH,
    'ffmpeg',
    ...(isWin
      ? [
          ...findWingetBinaries('ffmpeg'),
          'C:\\ProgramData\\chocolatey\\bin\\ffmpeg.exe',
          'C:\\ffmpeg\\bin\\ffmpeg.exe'
        ]
      : ['/opt/homebrew/bin/ffmpeg', '/usr/local/bin/ffmpeg', '/usr/bin/ffmpeg'])
  ].filter(Boolean)

  let ffmpegPath = null
  for (const candidate of ffmpegCandidates) {
    if (probeBinary(candidate)) {
      ffmpegPath = candidate
      break
    }
  }

  let ffprobePath = null
  const ffprobeCandidates = [
    process.env.FFPROBE_PATH,
    ffmpegPath ? join(resolve(ffmpegPath, '..'), `ffprobe${exeSuffix}`) : null,
    'ffprobe',
    ...(isWin ? findWingetBinaries('ffprobe') : ['/opt/homebrew/bin/ffprobe', '/usr/local/bin/ffprobe', '/usr/bin/ffprobe'])
  ].filter(Boolean)

  for (const candidate of ffprobeCandidates) {
    if (probeBinary(candidate)) {
      ffprobePath = candidate
      break
    }
  }

  return { ffmpegPath, ffprobePath }
}

export function probeVideoMetadata(ffprobePath, videoPath) {
  if (!ffprobePath) {
    return null
  }
  const result = spawnSync(
    ffprobePath,
    ['-v', 'quiet', '-print_format', 'json', '-show_format', '-show_streams', videoPath],
    { encoding: 'utf8', windowsHide: true }
  )
  if (result.error || result.status !== 0 || !result.stdout) {
    return null
  }
  try {
    const parsed = JSON.parse(result.stdout)
    const videoStream = (parsed.streams || []).find((stream) => stream.codec_type === 'video')
    const duration = Number(parsed.format?.duration || videoStream?.duration || 0)
    return {
      durationSec: Number.isFinite(duration) ? duration : 0,
      width: videoStream?.width || 0,
      height: videoStream?.height || 0,
      codec: videoStream?.codec_name || ''
    }
  } catch {
    return null
  }
}

export function runFfmpeg(ffmpegPath, args) {
  return spawnSync(ffmpegPath, args, {
    encoding: 'utf8',
    windowsHide: true,
    maxBuffer: 16 * 1024 * 1024
  })
}

export function extractVideoToFrames({
  ffmpegPath,
  videoPath,
  framesDir,
  fps,
  maxFrames,
  frameWidth
}) {
  mkdirSync(framesDir, { recursive: true })
  const rawDir = join(framesDir, '_raw')
  mkdirSync(rawDir, { recursive: true })

  const scaleFilter = frameWidth > 0 ? `,scale=${frameWidth}:-2:flags=lanczos` : ''
  const rawPattern = join(rawDir, 'frame_%06d.png')
  const extractResult = runFfmpeg(ffmpegPath, [
    '-y',
    '-hide_banner',
    '-loglevel',
    'error',
    '-i',
    videoPath,
    '-vf',
    `fps=${fps}${scaleFilter}`,
    rawPattern
  ])

  if (extractResult.error || extractResult.status !== 0) {
    throw new Error(
      `ffmpeg frame extraction failed: ${extractResult.stderr || extractResult.error?.message || 'unknown error'}`
    )
  }

  const rawFiles = readdirSync(rawDir)
    .filter((name) => IMAGE_EXTENSIONS.has(extname(name).toLowerCase()))
    .sort()
    .map((name) => join(rawDir, name))

  return normalizeFrameSet(rawFiles, framesDir, maxFrames, fps)
}

export function ingestFrameDirectory({ sourceDir, framesDir, maxFrames, assumedFps }) {
  mkdirSync(framesDir, { recursive: true })
  const sourceFiles = readdirSync(sourceDir)
    .filter((name) => IMAGE_EXTENSIONS.has(extname(name).toLowerCase()))
    .sort()
    .map((name) => join(sourceDir, name))

  return normalizeFrameSet(sourceFiles, framesDir, maxFrames, assumedFps)
}

function normalizeFrameSet(sourceFiles, framesDir, maxFrames, fps) {
  if (sourceFiles.length === 0) {
    return []
  }

  const selected = sampleEvenly(sourceFiles, maxFrames)
  const frames = []

  for (let i = 0; i < selected.length; i += 1) {
    const sourcePath = selected[i]
    const ext = extname(sourcePath).toLowerCase() || '.png'
    const indexLabel = String(i + 1).padStart(4, '0')
    const outPath = join(framesDir, `frame_${indexLabel}${ext}`)
    if (resolve(sourcePath) !== resolve(outPath)) {
      copyFileSync(sourcePath, outPath)
    }
    const stat = statSync(outPath)
    const approxTimeSec = Number(((i * (sourceFiles.length / selected.length)) / fps).toFixed(3))
    frames.push({
      index: i + 1,
      path: outPath,
      fileName: `frame_${indexLabel}${ext}`,
      sizeBytes: stat.size,
      approxTimeSec
    })
  }

  return frames
}

function sampleEvenly(items, maxCount) {
  if (items.length <= maxCount) {
    return items
  }
  if (maxCount <= 1) {
    return [items[0]]
  }
  const sampled = []
  const lastIndex = items.length - 1
  for (let i = 0; i < maxCount; i += 1) {
    const idx = Math.round((i * lastIndex) / (maxCount - 1))
    sampled.push(items[idx])
  }
  return sampled
}

export function buildContactSheets({
  ffmpegPath,
  frames,
  sheetsDir,
  cols,
  rows,
  tileWidth
}) {
  if (!ffmpegPath || frames.length === 0) {
    return []
  }
  mkdirSync(sheetsDir, { recursive: true })

  const perSheet = cols * rows
  const sheets = []

  for (let start = 0; start < frames.length; start += perSheet) {
    const chunk = frames.slice(start, start + perSheet)
    const sheetIndex = Math.floor(start / perSheet) + 1
    const sheetName = `sheet_${String(sheetIndex).padStart(3, '0')}.png`
    const sheetPath = join(sheetsDir, sheetName)
    const actualRows = Math.max(1, Math.ceil(chunk.length / cols))
    const inputArgs = []
    const filterParts = []

    for (let i = 0; i < chunk.length; i += 1) {
      inputArgs.push('-i', chunk[i].path)
      filterParts.push(`[${i}:v]scale=${tileWidth}:-2:flags=lanczos[t${i}]`)
    }

    const concatInputs = chunk.map((_, i) => `[t${i}]`).join('')
    const filterGraph = `${filterParts.join(';')};${concatInputs}concat=n=${chunk.length}:v=1:a=0,tile=${cols}x${actualRows}:padding=4:margin=4:color=0x16181d[out]`

    const result = runFfmpeg(ffmpegPath, [
      '-y',
      '-hide_banner',
      '-loglevel',
      'error',
      ...inputArgs,
      '-filter_complex',
      filterGraph,
      '-map',
      '[out]',
      '-frames:v',
      '1',
      sheetPath
    ])

    if (result.error || result.status !== 0 || !existsSync(sheetPath)) {
      continue
    }

    sheets.push({
      index: sheetIndex,
      path: sheetPath,
      fileName: sheetName,
      frameStart: chunk[0].index,
      frameEnd: chunk[chunk.length - 1].index,
      frameCount: chunk.length
    })
  }

  return sheets
}

export function detectVideoAnomalies({ ffmpegPath, videoPath, frames }) {
  const anomalies = []

  if (ffmpegPath && videoPath && existsSync(videoPath)) {
    const detectResult = runFfmpeg(ffmpegPath, [
      '-hide_banner',
      '-i',
      videoPath,
      '-vf',
      'blackdetect=d=0.25:pix_th=0.10,freezedetect=n=-60dB:d=1.5',
      '-an',
      '-f',
      'null',
      '-'
    ])
    const logText = `${detectResult.stderr || ''}\n${detectResult.stdout || ''}`
    for (const line of logText.split(/\r?\n/)) {
      if (line.includes('black_start:')) {
        anomalies.push({ type: 'black_frames', detail: line.trim() })
      } else if (line.includes('lavfi.freezedetect.freeze_start')) {
        anomalies.push({ type: 'frozen_segment', detail: line.trim() })
      }
    }
  }

  if (frames.length > 0) {
    const sizes = frames.map((frame) => frame.sizeBytes)
    const minSize = Math.min(...sizes)
    const maxSize = Math.max(...sizes)
    const avgSize = sizes.reduce((sum, v) => sum + v, 0) / sizes.length

    for (const frame of frames) {
      if (frame.sizeBytes < Math.max(1200, avgSize * 0.18)) {
        anomalies.push({
          type: 'low_detail_or_blank_frame',
          frameIndex: frame.index,
          fileName: frame.fileName,
          path: frame.path,
          sizeBytes: frame.sizeBytes,
          avgSizeBytes: Math.round(avgSize)
        })
      }
    }

    if (maxSize > 0 && minSize === maxSize && frames.length > 3) {
      anomalies.push({
        type: 'identical_frame_sizes',
        detail: `All ${frames.length} frames are ${minSize} bytes`
      })
    }

    if (frames.length >= 3) {
      const hashCounts = new Map()
      for (const frame of frames) {
        if (!existsSync(frame.path)) {
          continue
        }
        const digest = createHash('sha256').update(readFileSync(frame.path)).digest('hex')
        hashCounts.set(digest, (hashCounts.get(digest) || 0) + 1)
      }
      const uniqueCount = hashCounts.size
      const dominantCount = Math.max(0, ...hashCounts.values())
      if (uniqueCount <= 1) {
        anomalies.push({
          type: 'static_screen',
          detail: `All ${frames.length} captured frames are identical — nothing changed on screen (bot failed to drive the app or app froze)`
        })
      } else if (frames.length >= 6 && uniqueCount <= 2 && dominantCount / frames.length >= 0.9) {
        anomalies.push({
          type: 'mostly_static_screen',
          detail: `${dominantCount}/${frames.length} frames are identical (${uniqueCount} unique frames) — bot likely stalled or lost focus`
        })
      }
    }
  }

  return anomalies
}

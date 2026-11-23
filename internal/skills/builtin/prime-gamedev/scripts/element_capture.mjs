#!/usr/bin/env node
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { parseFlagArgs, resolveOutputDirectory } from './lib/args.mjs'
import { resolveFfmpegBinaries, runFfmpeg } from './lib/ffmpeg.mjs'
import { launchGameBrowser, loadPlaywright } from './lib/playwright_loader.mjs'

function sanitizeLabel(value) {
  return String(value || 'element')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '_')
    .replace(/^_+|_+$/g, '') || 'element'
}

export function parseCropSpecs(raw) {
  if (!raw) {
    return []
  }
  return String(raw)
    .split(';')
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item, idx) => {
      const colonIdx = item.indexOf(':')
      const label = colonIdx >= 0 ? sanitizeLabel(item.slice(0, colonIdx)) : `crop_${idx + 1}`
      const coordsRaw = colonIdx >= 0 ? item.slice(colonIdx + 1) : item
      const parts = coordsRaw.split(',').map((part) => Number(part.trim()))
      if (parts.length !== 4 || parts.some((num) => !Number.isFinite(num))) {
        throw new Error(`Invalid crop spec "${item}". Expected label:x,y,width,height`)
      }
      return {
        label,
        x: Math.max(0, Math.round(parts[0])),
        y: Math.max(0, Math.round(parts[1])),
        width: Math.max(1, Math.round(parts[2])),
        height: Math.max(1, Math.round(parts[3]))
      }
    })
}

export function cropImageWithFfmpeg({ imagePath, crops, outDir, scale = 2 }) {
  const source = resolve(imagePath)
  if (!existsSync(source)) {
    throw new Error(`Source image not found: ${source}`)
  }
  const { ffmpegPath } = resolveFfmpegBinaries()
  if (!ffmpegPath) {
    throw new Error('ffmpeg not found in PATH')
  }
  mkdirSync(outDir, { recursive: true })
  const scaleFactor = Math.max(1, Math.min(4, Number(scale) || 2))
  const captures = []

  for (const crop of crops) {
    const fileName = `${crop.label}.png`
    const outPath = join(outDir, fileName)
    const filter = `crop=${crop.width}:${crop.height}:${crop.x}:${crop.y},scale=iw*${scaleFactor}:ih*${scaleFactor}:flags=lanczos`
    const res = runFfmpeg(ffmpegPath, [
      '-y',
      '-hide_banner',
      '-loglevel',
      'error',
      '-i',
      source,
      '-vf',
      filter,
      outPath
    ])
    if (res.status !== 0 || !existsSync(outPath)) {
      throw new Error(`Failed to crop ${crop.label}: ${res.stderr || res.stdout}`)
    }
    captures.push({
      label: crop.label,
      kind: 'crop',
      box: { x: crop.x, y: crop.y, width: crop.width, height: crop.height },
      scale: scaleFactor,
      fileName,
      path: outPath
    })
  }
  return captures
}

export async function captureWebElements({
  url,
  outDir,
  selectors = [],
  crops = [],
  padding = 16,
  width = 1280,
  height = 720,
  deviceScaleFactor = 2,
  waitMs = 800,
  headed = false,
  playwrightPath = ''
}) {
  mkdirSync(outDir, { recursive: true })
  const { chromium } = await loadPlaywright(playwrightPath)
  const browser = await launchGameBrowser(chromium, { headed })
  const captures = []

  try {
    const context = await browser.newContext({
      viewport: { width, height },
      deviceScaleFactor
    })
    const page = await context.newPage()
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForTimeout(waitMs)

    for (let i = 0; i < selectors.length; i += 1) {
      const rawEntry = selectors[i]
      const sep = rawEntry.indexOf('=')
      const label = sep > 0 ? sanitizeLabel(rawEntry.slice(0, sep)) : `element_${i + 1}`
      const selector = sep > 0 ? rawEntry.slice(sep + 1).trim() : rawEntry.trim()
      if (!selector) {
        continue
      }
      const locator = page.locator(selector).first()
      await locator.waitFor({ state: 'visible', timeout: 8000 })
      const box = await locator.boundingBox()
      if (!box || box.width <= 0 || box.height <= 0) {
        continue
      }
      const clip = {
        x: Math.max(0, Math.floor(box.x - padding)),
        y: Math.max(0, Math.floor(box.y - padding)),
        width: Math.min(width - Math.max(0, Math.floor(box.x - padding)), Math.ceil(box.width + padding * 2)),
        height: Math.min(height - Math.max(0, Math.floor(box.y - padding)), Math.ceil(box.height + padding * 2))
      }
      const fileName = `${label}.png`
      const filePath = join(outDir, fileName)
      await page.screenshot({ path: filePath, clip })
      captures.push({
        label,
        kind: 'selector',
        selector,
        clip,
        fileName,
        path: filePath
      })
    }

    for (const crop of crops) {
      const clip = {
        x: Math.max(0, crop.x),
        y: Math.max(0, crop.y),
        width: Math.min(width - Math.max(0, crop.x), crop.width),
        height: Math.min(height - Math.max(0, crop.y), crop.height)
      }
      if (clip.width <= 0 || clip.height <= 0) {
        continue
      }
      const fileName = `${crop.label}.png`
      const filePath = join(outDir, fileName)
      await page.screenshot({ path: filePath, clip })
      captures.push({
        label: crop.label,
        kind: 'crop',
        clip,
        fileName,
        path: filePath
      })
    }

    await context.close()
  } finally {
    await browser.close()
  }

  return captures
}

function printUsage() {
  console.log(`Usage:
  node element_capture.mjs --url <http://...> [--selectors "hud=.hud;player=#player"] [--crops "boss:300,200,180,180"] [--padding 16] [--zoom 2] [--out <dir>]
  node element_capture.mjs --image <frame.png> --crops "player:100,120,180,180;hud:0,0,400,80" [--scale 2] [--out <dir>]`)
}

async function main() {
  const { flags } = parseFlagArgs(process.argv.slice(2))
  if (flags.help || (!flags.url && !flags.image)) {
    printUsage()
    process.exit(flags.help ? 0 : 1)
  }

  const outDir = resolveOutputDirectory(flags.out, 'prime-gamedev-elements')
  const crops = parseCropSpecs(flags.crops)
  let captures = []

  if (flags.image) {
    if (crops.length === 0) {
      throw new Error('--crops is required when using --image')
    }
    captures = cropImageWithFfmpeg({
      imagePath: flags.image,
      crops,
      outDir,
      scale: Number(flags.scale) || 2
    })
  } else {
    const selectors = String(flags.selectors || '')
      .split(';')
      .map((s) => s.trim())
      .filter(Boolean)
    captures = await captureWebElements({
      url: flags.url,
      outDir,
      selectors,
      crops,
      padding: Number(flags.padding) || 16,
      width: Number(flags.width) || 1280,
      height: Number(flags.height) || 720,
      deviceScaleFactor: Number(flags.zoom) || 2,
      waitMs: Number(flags.waitMs) || 800,
      headed: Boolean(flags.headed),
      playwrightPath: flags.playwrightPath || ''
    })
  }

  const manifestPath = join(outDir, 'elements_manifest.json')
  writeFileSync(manifestPath, JSON.stringify({ outDir, count: captures.length, captures }, null, 2))
  console.log(`Captured ${captures.length} element close-ups in: ${outDir}`)
  console.log(`Manifest: ${manifestPath}`)
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    console.error(error?.stack || error?.message || error)
    process.exit(1)
  })
}

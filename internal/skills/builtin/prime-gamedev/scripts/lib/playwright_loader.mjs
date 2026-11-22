import { existsSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))

const PLAYWRIGHT_PACKAGES = [
  ['node_modules', 'playwright', 'index.mjs'],
  ['node_modules', 'playwright', 'index.js'],
  ['node_modules', 'playwright-core', 'index.mjs'],
  ['node_modules', 'playwright-core', 'index.js'],
  ['node_modules', '@playwright', 'test', 'index.mjs'],
  ['node_modules', '@playwright', 'test', 'index.js']
]

function findPackagePlaywright(startDir) {
  let current = resolve(startDir)
  while (true) {
    for (const segments of PLAYWRIGHT_PACKAGES) {
      const candidate = join(current, ...segments)
      if (existsSync(candidate)) {
        return candidate
      }
    }
    const parent = dirname(current)
    if (parent === current) {
      return null
    }
    current = parent
  }
}

export async function loadPlaywright(explicitPath) {
  const candidates = []

  if (explicitPath) {
    candidates.push(resolve(explicitPath))
  }

  const fromScript = findPackagePlaywright(scriptDir)
  if (fromScript) {
    candidates.push(fromScript)
  }

  const fromCwd = findPackagePlaywright(process.cwd())
  if (fromCwd) {
    candidates.push(fromCwd)
  }

  const siblingDesignerDir = resolve(scriptDir, '..', '..', '..', 'prime-designer', 'scripts')
  const fromSibling = findPackagePlaywright(siblingDesignerDir)
  if (fromSibling) {
    candidates.push(fromSibling)
  }

  for (const candidate of candidates) {
    try {
      const mod = await import(pathToFileURL(candidate).href)
      if (mod?.chromium) {
        return mod
      }
      if (mod?.default?.chromium) {
        return mod.default
      }
    } catch {
    }
  }

  for (const pkgName of ['playwright', 'playwright-core', '@playwright/test']) {
    try {
      const mod = await import(pkgName)
      if (mod?.chromium) {
        return mod
      }
      if (mod?.default?.chromium) {
        return mod.default
      }
    } catch {
    }
  }

  try {
    const req = createRequire(join(process.cwd(), 'package.json'))
    for (const pkgName of ['playwright', 'playwright-core', '@playwright/test']) {
      try {
        const resolved = req.resolve(pkgName)
        const mod = await import(pathToFileURL(resolved).href)
        if (mod?.chromium) {
          return mod
        }
        if (mod?.default?.chromium) {
          return mod.default
        }
      } catch {
      }
    }
  } catch {
  }

  throw new Error(
    'Playwright is not installed. Install playwright or pass --playwright-path <path-to-playwright>.'
  )
}

export async function launchGameBrowser(chromium, { headed = false } = {}) {
  const launchArgs = [
    '--use-gl=angle',
    '--use-angle=swiftshader',
    '--enable-webgl',
    '--ignore-gpu-blocklist',
    '--autoplay-policy=no-user-gesture-required'
  ]

  const attempts = [
    { headless: !headed, args: launchArgs },
    { headless: !headed, channel: 'chrome', args: launchArgs },
    { headless: !headed, channel: 'msedge', args: launchArgs }
  ]

  let lastError = null
  for (const options of attempts) {
    try {
      return await chromium.launch(options)
    } catch (error) {
      lastError = error
    }
  }

  throw new Error(
    `Failed to launch Chromium for game bot: ${lastError?.message || 'unknown browser launch error'}`
  )
}

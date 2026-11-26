#!/usr/bin/env node
import { copyFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { basename, dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { parseFlagArgs } from './lib/args.mjs'

const SCOPE_TO_FOLDER = {
  feature: 'features',
  physics: 'physics',
  engine: 'engines',
  element: 'elements',
  game: 'games'
}

const VALID_SCOPES = new Set(Object.keys(SCOPE_TO_FOLDER))

function getBuiltinPresets() {
  const presetsDir = join(dirname(fileURLToPath(import.meta.url)), 'presets')
  const definitions = [
    {
      id: 'preset:element:showcase-capture-probe',
      scope: 'element',
      category: 'asset-showcase',
      feature: 'showcase-visuals',
      tags: ['showcase', 'visual', 'assets', 'models', 'textures', 'weapons', 'multi-angle', 'photos'],
      fileName: 'showcase_capture_probe.mjs',
      summary:
        'Rapid multi-angle photo capture on a dedicated Showcase scene: iterates every standalone created and downloaded element (characters, weapons, props, structures) from multiple angles for visual review.'
    },
    {
      id: 'preset:physics:collision-suite-probe',
      scope: 'physics',
      category: 'collisions',
      physics: 'collisions',
      feature: 'collision-tests',
      tags: ['collision', 'unit-test', 'all-sides', 'collider', 'hitboxes', 'floor', 'walls', 'tunneling'],
      fileName: 'collision_suite_probe.mjs',
      summary:
        'Deterministic code-level collision test harness: verifies every solid object has a collider matching its visual bounds and blocks movement from all 4/6 sides without teleporting in-game.'
    },
    {
      id: 'preset:feature:weapon-crosshair-probe',
      scope: 'feature',
      category: 'weapons-crosshair',
      feature: 'weapons',
      physics: 'raycast',
      tags: ['weapon', 'crosshair', 'reticle', 'aim', 'shoot', 'ammo', 'reload', 'combat'],
      fileName: 'weapon_crosshair_probe.mjs',
      summary:
        'Verifies mandatory weapon crosshair visibility/placement, firing, ammo consumption, reload loop, and captures close-ups.'
    },
    {
      id: 'preset:physics:physics-bounds-probe',
      scope: 'physics',
      category: 'bounds-movement',
      physics: 'movement',
      feature: 'controls',
      tags: ['physics', 'movement', 'diagonal', 'bounds', 'gravity', 'timestep', 'nan'],
      fileName: 'physics_bounds_probe.mjs',
      summary:
        'Checks normalized diagonal movement speed, map bounds, jump/gravity stability, and zero NaN/Infinity coordinates.'
    },
    {
      id: 'preset:feature:balance-and-state-probe',
      scope: 'feature',
      category: 'balance-state',
      feature: 'balance',
      tags: ['balance', 'difficulty', 'pause', 'restart', 'state-reset', 'hp', 'fps', 'menu'],
      fileName: 'balance_and_state_probe.mjs',
      summary:
        'Verifies Menu -> Gameplay -> Pause -> Restart state lifecycle, full score/state reset, and records difficulty/FPS samples.'
    }
  ]

  return definitions
    .map((item) => {
      const absolutePath = join(presetsDir, item.fileName)
      if (!existsSync(absolutePath)) {
        return null
      }
      return {
        ...item,
        source: 'builtin-preset',
        relativePath: `presets/${item.fileName}`,
        absolutePath
      }
    })
    .filter(Boolean)
}

function slugify(value) {
  return String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

export function resolveLibraryRoot(explicitRoot) {
  if (explicitRoot) {
    return resolve(explicitRoot)
  }
  if (process.env.PRIME_GAMEDEV_LIBRARY) {
    return resolve(process.env.PRIME_GAMEDEV_LIBRARY)
  }
  const scriptDir = dirname(fileURLToPath(import.meta.url))
  const skillDir = resolve(scriptDir, '..')
  const parentDir = resolve(skillDir, '..')
  if (basename(parentDir) === 'builtin-skills') {
    return join(resolve(parentDir, '..'), 'gamedev-library')
  }
  if (process.platform === 'win32') {
    const localAppData = process.env.LOCALAPPDATA || join(homedir(), 'AppData', 'Local')
    return join(localAppData, 'prime', 'gamedev-library')
  }
  if (process.platform === 'darwin') {
    return join(homedir(), 'Library', 'Application Support', 'prime', 'gamedev-library')
  }
  const xdgData = process.env.XDG_DATA_HOME || join(homedir(), '.local', 'share')
  return join(xdgData, 'prime', 'gamedev-library')
}

export function ensureLibraryInitialized(libraryRoot) {
  for (const folder of Object.values(SCOPE_TO_FOLDER)) {
    mkdirSync(join(libraryRoot, folder), { recursive: true })
  }
  const indexPath = join(libraryRoot, 'index.json')
  if (!existsSync(indexPath)) {
    writeFileSync(indexPath, JSON.stringify({ version: 2, entries: [] }, null, 2))
  }
  return indexPath
}

export function readLibraryIndex(libraryRoot) {
  const indexPath = ensureLibraryInitialized(libraryRoot)
  try {
    const parsed = JSON.parse(readFileSync(indexPath, 'utf8'))
    if (Array.isArray(parsed.entries)) {
      return parsed
    }
  } catch {
  }
  return { version: 2, entries: [] }
}

function writeLibraryIndex(libraryRoot, data) {
  const indexPath = ensureLibraryInitialized(libraryRoot)
  writeFileSync(indexPath, JSON.stringify(data, null, 2))
  return indexPath
}

function scoreEntry(entry, criteria) {
  let score = 0
  const feature = slugify(criteria.feature)
  const physics = slugify(criteria.physics)
  const game = slugify(criteria.game)
  const engine = slugify(criteria.engine)
  const scope = slugify(criteria.scope)
  const queryTerms = String(criteria.query || '')
    .toLowerCase()
    .split(/[\s,]+/)
    .map((term) => term.trim())
    .filter(Boolean)

  if (scope && entry.scope !== scope) {
    return -1
  }
  if (feature) {
    if (entry.feature === feature || entry.category === feature) {
      score += 110
    } else if (Array.isArray(entry.tags) && entry.tags.some((tag) => tag.includes(feature))) {
      score += 60
    }
  }
  if (physics) {
    if (entry.physics === physics || entry.category === physics) {
      score += 110
    } else if (Array.isArray(entry.tags) && entry.tags.some((tag) => tag.includes(physics))) {
      score += 60
    }
  }
  if (game && entry.gameSlug === game) {
    score += 90
  }
  if (engine && entry.engine === engine) {
    score += 60
  }
  if (queryTerms.length > 0) {
    const haystack = [
      entry.id,
      entry.scope,
      entry.category,
      entry.feature,
      entry.physics,
      entry.engine,
      entry.gameSlug,
      entry.summary,
      ...(Array.isArray(entry.tags) ? entry.tags : [])
    ]
      .join(' ')
      .toLowerCase()
    for (const term of queryTerms) {
      if (haystack.includes(term)) {
        score += 25
      }
    }
  }
  if (score > 0 && entry.source !== 'builtin-preset') {
    score += 5
  }
  return score
}

export function lookupLibraryEntries(libraryRoot, criteria = {}) {
  const index = readLibraryIndex(libraryRoot)
  const limit = Math.max(1, Number(criteria.limit) || 6)
  const includePresets = criteria.includePresets !== false
  const hasFilters = Boolean(
    criteria.feature || criteria.physics || criteria.game || criteria.engine || criteria.scope || criteria.query
  )

  const userEntries = index.entries.map((entry) => ({
    ...entry,
    source: 'user-library',
    absolutePath: join(libraryRoot, entry.relativePath)
  }))
  const presets = includePresets ? getBuiltinPresets() : []
  const pool = [...userEntries, ...presets]

  const ranked = pool
    .map((entry) => ({
      entry,
      score: hasFilters ? scoreEntry(entry, criteria) : 1
    }))
    .filter((item) => item.score > 0 || !hasFilters)
    .sort((a, b) => b.score - a.score || String(b.entry.updatedAt || '').localeCompare(String(a.entry.updatedAt || '')))
    .slice(0, limit)
    .map(({ entry, score }) => ({
      ...entry,
      score
    }))

  return {
    libraryRoot,
    totalUserEntries: userEntries.length,
    totalBuiltinPresets: presets.length,
    totalEntries: userEntries.length + presets.length,
    matches: ranked
  }
}

export function saveLibraryEntry(libraryRoot, options) {
  const scope = slugify(options.scope)
  if (!VALID_SCOPES.has(scope)) {
    throw new Error('--scope must be one of: feature, physics, engine, element, game')
  }
  const sourceFile = resolve(options.file || '')
  if (!options.file || !existsSync(sourceFile)) {
    throw new Error(`Source script not found: ${options.file || ''}`)
  }
  const rawName = options.name || basename(sourceFile)
  const safeName = rawName.replace(/[<>:"/\\|?*]+/g, '_')
  const feature = slugify(options.feature)
  const physics = slugify(options.physics)
  const engine = slugify(options.engine)
  const gameSlug = slugify(options.game)
  const fallbackCategory =
    scope === 'feature'
      ? feature
      : scope === 'physics'
        ? physics
        : scope === 'game'
          ? gameSlug
          : engine
  const category = slugify(options.category || fallbackCategory || 'general')
  if (!category) {
    throw new Error('--category (or matching --feature / --physics / --engine / --game) is required')
  }

  const folderName = SCOPE_TO_FOLDER[scope]
  const relativeDir = join(folderName, category)
  const targetDir = join(libraryRoot, relativeDir)
  mkdirSync(targetDir, { recursive: true })
  const destination = join(targetDir, safeName)
  copyFileSync(sourceFile, destination)

  const relativePath = join(relativeDir, safeName).replace(/\\/g, '/')
  const id = `${scope}:${category}:${safeName}`
  const tags = String(options.tags || '')
    .split(',')
    .map((tag) => slugify(tag))
    .filter(Boolean)

  const record = {
    id,
    scope,
    category,
    feature: feature || (scope === 'feature' ? category : undefined),
    physics: physics || (scope === 'physics' ? category : undefined),
    engine: engine || undefined,
    gameSlug: gameSlug || undefined,
    tags,
    relativePath,
    summary: String(options.summary || `${scope} script ${safeName}`).trim(),
    updatedAt: new Date().toISOString()
  }

  const index = readLibraryIndex(libraryRoot)
  const existingIndex = index.entries.findIndex((item) => item.id === id)
  if (existingIndex >= 0) {
    index.entries[existingIndex] = record
  } else {
    index.entries.push(record)
  }
  writeLibraryIndex(libraryRoot, index)

  return {
    libraryRoot,
    savedPath: destination,
    record
  }
}

function printUsage() {
  console.log(`Usage:
  node library.mjs init [--library <dir>]
  node library.mjs lookup [--library <dir>] [--feature <name>] [--physics <topic>] [--engine <name>] [--game <slug>] [--scope feature|physics|engine|element|game] [--query "<terms>"] [--limit 6]
  node library.mjs save --scope feature|physics|engine|element|game --file <path> --summary "<one line>" [--category <name>] [--feature <name>] [--physics <topic>] [--engine <name>] [--game <slug>] [--name <filename>] [--tags "a,b"]`)
}

function main() {
  const { flags, positional } = parseFlagArgs(process.argv.slice(2))
  const command = positional[0]
  if (!command || flags.help) {
    printUsage()
    process.exit(flags.help ? 0 : 1)
  }
  const libraryRoot = resolveLibraryRoot(flags.library)

  if (command === 'init') {
    const indexPath = ensureLibraryInitialized(libraryRoot)
    console.log(JSON.stringify({ libraryRoot, indexPath, builtinPresets: getBuiltinPresets().length }, null, 2))
    return
  }

  if (command === 'lookup') {
    const result = lookupLibraryEntries(libraryRoot, {
      feature: flags.feature,
      physics: flags.physics,
      engine: flags.engine,
      game: flags.game,
      scope: flags.scope,
      query: flags.query,
      limit: flags.limit
    })
    console.log(JSON.stringify(result, null, 2))
    return
  }

  if (command === 'save') {
    const result = saveLibraryEntry(libraryRoot, {
      scope: flags.scope,
      category: flags.category,
      feature: flags.feature,
      physics: flags.physics,
      engine: flags.engine,
      game: flags.game,
      file: flags.file,
      name: flags.name,
      summary: flags.summary,
      tags: flags.tags
    })
    console.log(JSON.stringify(result, null, 2))
    return
  }

  throw new Error(`Unknown command: ${command}`)
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    main()
  } catch (error) {
    console.error(error?.message || error)
    process.exit(1)
  }
}

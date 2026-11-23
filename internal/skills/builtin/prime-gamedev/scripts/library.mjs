#!/usr/bin/env node
import { copyFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { basename, dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { parseFlagArgs } from './lib/args.mjs'

const VALID_SCOPES = new Set(['engine', 'game', 'element'])

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
  mkdirSync(join(libraryRoot, 'engines'), { recursive: true })
  mkdirSync(join(libraryRoot, 'games'), { recursive: true })
  mkdirSync(join(libraryRoot, 'elements'), { recursive: true })
  const indexPath = join(libraryRoot, 'index.json')
  if (!existsSync(indexPath)) {
    writeFileSync(indexPath, JSON.stringify({ version: 1, entries: [] }, null, 2))
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
  return { version: 1, entries: [] }
}

function writeLibraryIndex(libraryRoot, data) {
  const indexPath = ensureLibraryInitialized(libraryRoot)
  writeFileSync(indexPath, JSON.stringify(data, null, 2))
  return indexPath
}

function scoreEntry(entry, criteria) {
  let score = 0
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
  if (game && entry.gameSlug === game) {
    score += 100
  }
  if (engine && entry.engine === engine) {
    score += 60
  }
  if (entry.scope === 'element') {
    score += 10
  }
  if (queryTerms.length > 0) {
    const haystack = [
      entry.id,
      entry.category,
      entry.engine,
      entry.gameSlug,
      entry.summary,
      ...(Array.isArray(entry.tags) ? entry.tags : [])
    ]
      .join(' ')
      .toLowerCase()
    for (const term of queryTerms) {
      if (haystack.includes(term)) {
        score += 20
      }
    }
  }
  return score
}

export function lookupLibraryEntries(libraryRoot, criteria = {}) {
  const index = readLibraryIndex(libraryRoot)
  const limit = Math.max(1, Number(criteria.limit) || 5)
  const hasFilters = Boolean(criteria.game || criteria.engine || criteria.scope || criteria.query)
  const ranked = index.entries
    .map((entry) => ({
      entry,
      score: hasFilters ? scoreEntry(entry, criteria) : 1
    }))
    .filter((item) => item.score > 0 || !hasFilters)
    .sort((a, b) => b.score - a.score || String(b.entry.updatedAt).localeCompare(String(a.entry.updatedAt)))
    .slice(0, limit)
    .map(({ entry, score }) => ({
      ...entry,
      score,
      absolutePath: join(libraryRoot, entry.relativePath)
    }))

  return {
    libraryRoot,
    totalEntries: index.entries.length,
    matches: ranked
  }
}

export function saveLibraryEntry(libraryRoot, options) {
  const scope = slugify(options.scope)
  if (!VALID_SCOPES.has(scope)) {
    throw new Error('--scope must be one of: engine, game, element')
  }
  const sourceFile = resolve(options.file || '')
  if (!options.file || !existsSync(sourceFile)) {
    throw new Error(`Source script not found: ${options.file || ''}`)
  }
  const rawName = options.name || basename(sourceFile)
  const safeName = rawName.replace(/[<>:"/\\|?*]+/g, '_')
  const engine = slugify(options.engine)
  const gameSlug = slugify(options.game)
  const category = slugify(options.category || (scope === 'game' ? gameSlug : engine) || 'general')
  if (!category) {
    throw new Error('--category (or --game / --engine) is required')
  }

  const folderName = scope === 'engine' ? 'engines' : scope === 'game' ? 'games' : 'elements'
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
  node library.mjs lookup [--library <dir>] [--engine <name>] [--game <slug>] [--scope engine|game|element] [--query "<terms>"] [--limit 5]
  node library.mjs save --scope engine|game|element --file <path> --summary "<one line>" [--category <name>] [--engine <name>] [--game <slug>] [--name <filename>] [--tags "a,b"]`)
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
    console.log(JSON.stringify({ libraryRoot, indexPath }, null, 2))
    return
  }

  if (command === 'lookup') {
    const result = lookupLibraryEntries(libraryRoot, {
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

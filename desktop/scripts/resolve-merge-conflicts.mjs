import { execFileSync, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, posix } from 'node:path'
import { crateName, readVersion, versionFiles, writeVersion } from './native-version.mjs'

const bufferSize = 256 * 1024 * 1024
const git = (...args) => execFileSync('git', args, { encoding: 'utf8', maxBuffer: bufferSize, stdio: ['ignore', 'pipe', 'ignore'] })
const root = git('rev-parse', '--show-toplevel').trim()
const managed = new Set(versionFiles.map(file => `desktop/${file}`))

const tryGit = (...args) => {
  try {
    return git('-C', root, ...args)
  } catch {
    return undefined
  }
}

const stage = (number, path) => tryGit('show', `:${number}:${path}`)

const conflicted = () => git('-C', root, 'diff', '--name-only', '--diff-filter=U').split('\n').map(line => line.trim()).filter(Boolean)

function theirsAlreadyInHistory(path) {
  const blob = tryGit('rev-parse', `:3:${path}`)?.trim()
  if (!blob || tryGit('rev-parse', `:2:${path}`) === undefined) return false
  const commit = tryGit('log', '-1', '--format=%H', `--find-object=${blob}`, 'HEAD', '--', path)?.trim()
  return Boolean(commit)
}

const keepOurs = path => {
  git('-C', root, 'checkout', '--ours', '--', path)
  git('-C', root, 'add', '--', path)
}

const lockName = path => {
  if (!path.endsWith('Cargo.lock')) return undefined
  const manifestPath = posix.join(posix.dirname(path), 'Cargo.toml')
  const manifest = stage(2, manifestPath) ?? stage(0, manifestPath)
  return manifest ? crateName(manifest) : undefined
}

function mergeVersionFile(path, workDir) {
  const base = stage(1, path)
  const ours = stage(2, path)
  const theirs = stage(3, path)
  if (base === undefined || ours === undefined || theirs === undefined) return false
  const name = lockName(path)
  const version = readVersion(path, ours, name)
  if (!version) return false
  const sides = [base, ours, theirs].map((content, index) => {
    const file = join(workDir, `${index}`)
    writeFileSync(file, writeVersion(path, content, version, name))
    return file
  })
  const merged = spawnSync('git', ['merge-file', '-p', sides[1], sides[0], sides[2]], { encoding: 'utf8', maxBuffer: bufferSize })
  if (merged.status !== 0) return false
  writeFileSync(join(root, path), merged.stdout)
  git('-C', root, 'add', '--', path)
  return true
}

const workDir = mkdtempSync(join(tmpdir(), 'prime-merge-'))
try {
  for (const path of conflicted()) {
    if (theirsAlreadyInHistory(path)) {
      keepOurs(path)
      console.log(` Kept main's ${path}: the branch copy is an older main state`)
    } else if (managed.has(path) && mergeVersionFile(path, workDir)) {
      console.log(` Resolved the version conflict in ${path}`)
    }
  }
} finally {
  rmSync(workDir, { recursive: true, force: true })
}

const remaining = conflicted()
if (remaining.length > 0) {
  console.log(' Unresolved conflicts:')
  for (const path of remaining) console.log(`   ${path}`)
  process.exit(1)
}

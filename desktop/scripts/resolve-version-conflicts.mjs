import { execFileSync, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, posix } from 'node:path'
import { crateName, readVersion, versionFiles, writeVersion } from './native-version.mjs'

const git = (...args) => execFileSync('git', args, { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024 })
const root = git('rev-parse', '--show-toplevel').trim()
const managed = new Set(versionFiles.map(file => `desktop/${file}`))

const stage = (number, path) => {
  try {
    return execFileSync('git', ['show', `:${number}:${path}`], { cwd: root, encoding: 'utf8', maxBuffer: 256 * 1024 * 1024, stdio: ['ignore', 'pipe', 'ignore'] })
  } catch {
    return undefined
  }
}

const conflicted = () => git('-C', root, 'diff', '--name-only', '--diff-filter=U').split('\n').map(line => line.trim()).filter(Boolean)

const lockName = (path, side) => {
  if (!path.endsWith('Cargo.lock')) return undefined
  const manifest = stage(side, posix.join(posix.dirname(path), 'Cargo.toml')) ?? stage(0, posix.join(posix.dirname(path), 'Cargo.toml'))
  return manifest ? crateName(manifest) : undefined
}

function resolve(path, workDir) {
  const base = stage(1, path)
  const ours = stage(2, path)
  const theirs = stage(3, path)
  if (base === undefined || ours === undefined || theirs === undefined) return false
  const name = lockName(path, 2)
  const version = readVersion(path, ours, name)
  if (!version) return false
  const sides = [base, ours, theirs].map((content, index) => {
    const file = join(workDir, `${index}`)
    writeFileSync(file, writeVersion(path, content, version, name))
    return file
  })
  const merged = spawnSync('git', ['merge-file', '-p', sides[1], sides[0], sides[2]], { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024 })
  if (merged.status !== 0) return false
  writeFileSync(join(root, path), merged.stdout)
  git('-C', root, 'add', '--', path)
  return true
}

const workDir = mkdtempSync(join(tmpdir(), 'prime-version-merge-'))
try {
  for (const path of conflicted()) {
    if (!managed.has(path)) continue
    if (resolve(path, workDir)) console.log(` Resolved the version conflict in ${path}`)
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

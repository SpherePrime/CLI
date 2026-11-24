import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { crateName, versionFiles, writeVersion } from './native-version.mjs'

const parse = (value) => {
  const match = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(String(value).trim())
  return match ? match.slice(1).map(Number) : undefined
}
const newer = (left, right) => {
  for (let index = 0; index < 3; index++)
    if (left[index] !== right[index]) return left[index] > right[index] ? left : right
  return left
}

const desktopDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const packagePath = resolve(desktopDir, 'package.json')
const manifest = JSON.parse(readFileSync(packagePath, 'utf8'))
let base = parse(manifest.version) ?? [0, 0, 0]
try {
  const tags = execFileSync('git', ['tag', '--list', 'v*'], { cwd: desktopDir, encoding: 'utf8' }).split(/\r?\n/)
  for (const tag of tags) {
    const version = parse(tag)
    if (version) base = newer(base, version)
  }
} catch {}
const next = `${base[0]}.${base[1]}.${base[2] + 1}`

for (const relPath of versionFiles) {
  const fullPath = resolve(desktopDir, relPath)
  if (!existsSync(fullPath)) continue
  const name = relPath.endsWith('Cargo.lock')
    ? crateName(readFileSync(resolve(dirname(fullPath), 'Cargo.toml'), 'utf8'))
    : undefined
  writeFileSync(fullPath, writeVersion(relPath, readFileSync(fullPath, 'utf8'), next, name))
}
console.log(next)

import { execFileSync } from 'node:child_process'
import { readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const parse = (value) => {
  const match = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(String(value).trim())
  return match ? match.slice(1).map(Number) : undefined
}
const newer = (left, right) => {
  for (let index = 0; index < 3; index++)
    if (left[index] !== right[index]) return left[index] > right[index] ? left : right
  return left
}

const packagePath = resolve('package.json')
const lockPath = resolve('package-lock.json')
const manifest = JSON.parse(readFileSync(packagePath, 'utf8'))
let base = parse(manifest.version) ?? [0, 0, 0]
try {
  const tags = execFileSync('git', ['tag', '--list', 'v*'], { encoding: 'utf8' }).split(/\r?\n/)
  for (const tag of tags) {
    const version = parse(tag)
    if (version) base = newer(base, version)
  }
} catch {}
const next = `${base[0]}.${base[1]}.${base[2] + 1}`
const rewrite = (path, update) => {
  const text = readFileSync(path, 'utf8')
  const eol = text.includes('\r\n') ? '\r\n' : '\n'
  const value = JSON.parse(text)
  update(value)
  writeFileSync(path, JSON.stringify(value, null, 2).replace(/\n/g, eol) + eol)
}
rewrite(packagePath, (value) => (value.version = next))
try {
  rewrite(lockPath, (value) => {
    value.version = next
    if (value.packages?.['']) value.packages[''].version = next
  })
} catch {}
console.log(next)

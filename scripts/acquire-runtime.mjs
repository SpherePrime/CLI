import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
const root = fileURLToPath(new URL('..', import.meta.url))
const cli = process.env.PRIME_CLI_SOURCE || fileURLToPath(new URL('../../CLI/', import.meta.url))
const commit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: cli, encoding: 'utf8' }).trim()
if (execFileSync('git', ['status', '--porcelain'], { cwd: cli, encoding: 'utf8' }).trim()) throw new Error('Commit CLI changes before building a pinned runtime')
await mkdir(new URL('../runtime/', import.meta.url), { recursive: true })
const binary = fileURLToPath(new URL('../runtime/prime.exe', import.meta.url))
execFileSync('go', ['build', '-o', binary, '.'], { cwd: cli, stdio: 'inherit' })
const bytes = await readFile(binary)
await writeFile(
  new URL('../runtime/manifest.json', import.meta.url),
  JSON.stringify(
    {
      version: 'development',
      commit,
      platform: 'win32',
      arch: 'x64',
      protocolVersion: 1,
      sha256: createHash('sha256').update(bytes).digest('hex')
    },
    null,
    2
  )
)
console.log(`Runtime built from ${commit} in ${root}`)

import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { existsSync } from 'node:fs'
import { join, resolve } from 'node:path'
const root = fileURLToPath(new URL('..', import.meta.url))
const parent = resolve(root, '..')
const cli =
  process.env.PRIME_CLI_SOURCE ||
  (existsSync(join(parent, 'go.mod')) ? parent : join(parent, 'CLI'))
const commit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: cli, encoding: 'utf8' }).trim()
if (execFileSync('git', ['status', '--porcelain'], { cwd: cli, encoding: 'utf8' }).trim())
  throw new Error('Commit CLI changes before building a pinned runtime')
await mkdir(new URL('../runtime/', import.meta.url), { recursive: true })
const binary = fileURLToPath(new URL('../runtime/prime.exe', import.meta.url))
const releaseVersion = process.env.PRIME_RELEASE_VERSION
if (releaseVersion && !/^\d+\.\d+\.\d+$/.test(releaseVersion))
  throw new Error('Invalid release version')
execFileSync(
  'go',
  [
    'build',
    '-trimpath',
    '-buildvcs=false',
    ...(releaseVersion
      ? [
          '-ldflags',
          `-s -w -X github.com/SpherePrime/CLI/internal/version.Version=${releaseVersion}`
        ]
      : []),
    '-o',
    binary,
    '.'
  ],
  { cwd: cli, stdio: 'inherit', env: { ...process.env, CGO_ENABLED: '0' } }
)
const bytes = await readFile(binary)
await writeFile(
  new URL('../runtime/manifest.json', import.meta.url),
  JSON.stringify(
    {
      version: releaseVersion || 'development',
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

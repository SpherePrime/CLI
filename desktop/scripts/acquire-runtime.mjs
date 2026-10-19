import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { existsSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { runtimeTargets } from './runtime-targets.mjs'
const root = fileURLToPath(new URL('..', import.meta.url))
const parent = resolve(root, '..')
const cli =
  process.env.PRIME_CLI_SOURCE ||
  (existsSync(join(parent, 'go.mod')) ? parent : join(parent, 'CLI'))
const headCommit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: cli, encoding: 'utf8' }).trim()
const releaseVersion = process.env.PRIME_RELEASE_VERSION
if (releaseVersion && !/^\d+\.\d+\.\d+$/.test(releaseVersion))
  throw new Error('Invalid release version')
const dirty = Boolean(
  execFileSync('git', ['status', '--porcelain'], { cwd: cli, encoding: 'utf8' }).trim()
)
if (dirty && releaseVersion) throw new Error('Commit CLI changes before building a pinned runtime')
const commit = dirty ? `${headCommit}-dirty` : headCommit
const runtimeRoot = fileURLToPath(new URL('../runtime/', import.meta.url))
for (const target of runtimeTargets) {
  const directory = join(runtimeRoot, target.directory)
  await mkdir(directory, { recursive: true })
  const binary = join(directory, target.binary)
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
    {
      cwd: cli,
      stdio: 'inherit',
      env: { ...process.env, CGO_ENABLED: '0', GOOS: target.goos, GOARCH: target.goarch }
    }
  )
  const bytes = await readFile(binary)
  await writeFile(
    join(directory, 'manifest.json'),
    JSON.stringify(
      {
        version: releaseVersion || 'development',
        commit,
        platform: target.platform,
        arch: target.arch,
        protocolVersion: 1,
        sha256: createHash('sha256').update(bytes).digest('hex')
      },
      null,
      2
    )
  )
  console.log(`Runtime ${target.platform}-${target.arch} built from ${commit} in ${root}`)
}

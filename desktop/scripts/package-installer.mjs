import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'
import { linuxPayload, windowsPayload } from './installer-payload.mjs'

const root = fileURLToPath(new URL('..', import.meta.url))
const setup = join(root, 'installer')
const linux = process.platform === 'linux'
const online = process.argv.includes('--online')
const { version } = JSON.parse(await readFile(join(root, 'package.json'), 'utf8'))
const runtime = JSON.parse(await readFile(join(root, 'runtime/manifest.json'), 'utf8'))
const packageJson = JSON.parse(await readFile(join(setup, 'package.json'), 'utf8'))
packageJson.version = version
await writeFile(join(setup, 'package.json'), JSON.stringify(packageJson, null, 2) + '\n')
const release = join(root, 'release', version)
const payload = join(setup, 'payload')
await rm(payload, { recursive: true, force: true })
await mkdir(payload, { recursive: true })
const context = { release, payload, version, runtime, online }
const manifest = linux ? await linuxPayload(context) : await windowsPayload(context)
await writeFile(join(payload, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n')
const run = (file, args) =>
  execFileSync(process.execPath, [join(root, 'node_modules', file), ...args], {
    cwd: root,
    stdio: 'inherit'
  })
run('electron-vite/bin/electron-vite.js', [
  'build',
  '--config',
  'installer/electron.vite.config.ts'
])
run('electron-builder/cli.js', [
  '--projectDir',
  'installer',
  '--config',
  'electron-builder.cjs',
  ...(linux ? ['--linux', 'AppImage'] : ['--win', 'portable']),
  '--x64',
  '--publish',
  'never'
])

import { createHash } from 'node:crypto'
import { createReadStream } from 'node:fs'
import { copyFile, mkdir, readFile, readdir, stat, writeFile } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const root = fileURLToPath(new URL('..', import.meta.url))
const setup = join(root, 'installer')
const { version } = JSON.parse(await readFile(join(root, 'package.json'), 'utf8'))
const source = join(root, 'release', version, `PrimeDesktop-Update-${version}.exe`)
const runtime = JSON.parse(await readFile(join(root, 'runtime/manifest.json'), 'utf8'))
const packageJson = JSON.parse(await readFile(join(setup, 'package.json'), 'utf8'))
packageJson.version = version
await writeFile(join(setup, 'package.json'), JSON.stringify(packageJson, null, 2) + '\n')
const hash = createHash('sha256')
for await (const bytes of createReadStream(source)) hash.update(bytes)
let installedSize = 0
const unpacked = join(root, 'release', version, 'win-unpacked')
for (const entry of await readdir(unpacked, { recursive: true, withFileTypes: true }))
  if (entry.isFile()) installedSize += (await stat(join(entry.parentPath, entry.name))).size
const payload = join(setup, 'payload')
await mkdir(payload, { recursive: true })
await copyFile(source, join(payload, 'PrimeDesktopSetup.exe'))
await writeFile(
  join(payload, 'manifest.json'),
  JSON.stringify(
    { version, sha256: hash.digest('hex'), runtimeSha256: runtime.sha256, installedSize },
    null,
    2
  ) + '\n'
)
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
  '--win',
  'portable',
  '--x64',
  '--publish',
  'never'
])

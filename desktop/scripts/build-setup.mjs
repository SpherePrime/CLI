import { spawnSync } from 'node:child_process'
import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const release = join(root, 'src-tauri', 'target', 'release')
const bundle = join(release, 'bundle', 'nsis')
const version = JSON.parse(readFileSync(join(root, 'src-tauri', 'tauri.conf.json'), 'utf8')).version
const payload = readdirSync(bundle)
  .filter((name) => name.endsWith(`${version}_x64-setup.exe`))
  .map((name) => join(bundle, name))
  .sort((left, right) => statSync(right).mtimeMs - statSync(left).mtimeMs)[0]
if (!payload) throw new Error(`NSIS payload for ${version} was not built`)

function size(path) {
  const info = statSync(path)
  return info.isDirectory()
    ? readdirSync(path).reduce((total, name) => total + size(join(path, name)), 0)
    : info.size
}
const executable = ['PrimeDesktop.exe', 'prime-native.exe'].map((name) => join(release, name)).find(existsSync)
if (!executable) throw new Error('Release executable was not built')
const installedSize = size(executable) + size(join(release, 'engine'))

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', ...options })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}

run(process.execPath, [join(root, 'node_modules', 'vite', 'bin', 'vite.js'), 'build', '--config', 'installer/vite.config.ts'])
run(process.execPath, [join(root, 'node_modules', '@tauri-apps', 'cli', 'tauri.js'), 'build', '--no-bundle'], {
  cwd: join(root, 'installer'),
  env: {
    ...process.env,
    CARGO_TARGET_DIR: resolve(root, 'src-tauri', 'target'),
    PRIME_SETUP_PAYLOAD: payload,
    PRIME_SETUP_INSTALLED_SIZE: String(installedSize)
  }
})
const output = join(release, 'bundle', 'setup')
mkdirSync(output, { recursive: true })
const target = join(output, `PrimeDesktop-Setup-${version}.exe`)
copyFileSync(join(release, 'prime-setup.exe'), target)
console.log(target)

import { spawnSync } from 'node:child_process'
import { rmSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { resolveUpdaterKeyConfig, verifyUpdaterKeyPair } from './updater-key.mjs'

const root = fileURLToPath(new URL('..', import.meta.url))
const tag = process.argv[2]
if (!/^v\d+\.\d+\.\d+$/.test(tag || '')) throw new Error('A stable release tag is required')
if (process.platform !== 'win32') throw new Error('The local desktop release builds the Windows package only')

const keyConfig = resolveUpdaterKeyConfig(root)
if (!keyConfig.privateKey) {
  throw new Error(`Updater signing key not found. Put it in ${keyConfig.keyFile} or set TAURI_SIGNING_PRIVATE_KEY`)
}
if (!keyConfig.publicKey) {
  throw new Error('Updater public key is missing: set PRIME_UPDATER_PUBLIC_KEY or plugins.updater.pubkey')
}

const tauriCliPath = join(root, 'node_modules', '@tauri-apps', 'cli', 'tauri.js')
verifyUpdaterKeyPair({
  root,
  privateKey: keyConfig.privateKey,
  password: keyConfig.password,
  publicKey: keyConfig.publicKey,
  publicKeyFromFile: keyConfig.publicKeyFromFile,
  tauriCliPath
})

const environment = {
  ...process.env,
  TAURI_SIGNING_PRIVATE_KEY: keyConfig.privateKey,
  TAURI_SIGNING_PRIVATE_KEY_PASSWORD: keyConfig.password,
  PRIME_UPDATER_PUBLIC_KEY: keyConfig.publicKey,
  PRIME_UPDATER_ENDPOINT: keyConfig.endpoint
}

function run(script, ...args) {
  const result = spawnSync(process.execPath, [join(root, 'scripts', script), ...args], { cwd: root, env: environment, stdio: 'inherit' })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}

rmSync(join(root, 'src-tauri', 'target', 'release', 'bundle'), { recursive: true, force: true })
rmSync(join(root, 'release', 'native'), { recursive: true, force: true })
run('build-native-installer.mjs', 'nsis')
run('build-setup.mjs')
run('stage-native-release.mjs', tag)

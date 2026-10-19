import { randomUUID } from 'node:crypto'
import { build } from 'esbuild'
import { spawn } from 'node:child_process'
import electron from 'electron'
import { resolve } from 'node:path'
const output = resolve('.superpowers/workbench/window-control-native.cjs')
await build({
  entryPoints: ['tests/fixtures/window-control-native.ts'],
  bundle: true,
  platform: 'node',
  format: 'cjs',
  external: ['electron', 'koffi'],
  outfile: output,
  banner: { js: '(async () => {' },
  footer: { js: '})().catch(error => { console.error(error); process.exit(1) })' }
})
const env = { ...process.env }
delete env.ELECTRON_RUN_AS_NODE
const child = spawn(
  electron,
  [
    output,
    '--user-data-dir=' + resolve('.superpowers/workbench/window-native-profile-' + randomUUID())
  ],
  { env, stdio: 'inherit', windowsHide: false }
)
const timer = setTimeout(() => {
  child.kill()
  process.exitCode = 1
}, 30000)
child.once('exit', (code) => {
  clearTimeout(timer)
  process.exitCode = code ?? 1
})

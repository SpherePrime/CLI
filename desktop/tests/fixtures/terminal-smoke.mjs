import { build } from 'esbuild'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import electron from 'electron'

const directory = fileURLToPath(new URL('../../', import.meta.url))
const manager = resolve(directory, '.superpowers/workbench/terminal-manager.cjs')
await build({
  entryPoints: [resolve(directory, 'src/main/terminal/manager.ts')],
  outfile: manager,
  bundle: true,
  platform: 'node',
  format: 'cjs',
  external: ['node-pty', 'koffi'],
  logLevel: 'silent'
})
const fixture = resolve(directory, 'tests/fixtures/terminal-runtime.cjs')
const environment = { ...process.env, PRIME_TERMINAL_MANAGER: manager }
delete environment.ELECTRON_RUN_AS_NODE
for (const executable of [process.execPath, electron]) {
  const result = await promisify(execFile)(executable, [fixture], {
    cwd: directory,
    env: environment,
    windowsHide: true,
    timeout: 30000
  })
  process.stdout.write(result.stdout)
  process.stderr.write(result.stderr)
}

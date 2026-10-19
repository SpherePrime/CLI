import { createPackageWithOptions } from '@electron/asar'
import { build } from 'esbuild'
import { cp, mkdir, mkdtemp, writeFile } from 'node:fs/promises'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { join, resolve } from 'node:path'
import electron from 'electron'

const directory = await mkdtemp(resolve('.superpowers/workbench/terminal-package-'))
const source = join(directory, 'source')
await mkdir(join(source, 'node_modules'), { recursive: true })
await cp(resolve('node_modules/node-pty'), join(source, 'node_modules/node-pty'), {
  recursive: true,
  filter: (path) => !path.endsWith('.pdb')
})
await cp(resolve('node_modules/koffi'), join(source, 'node_modules/koffi'), { recursive: true })
await cp(resolve('node_modules/@koromix'), join(source, 'node_modules/@koromix'), { recursive: true })
await cp(resolve('tests/fixtures/terminal-runtime.cjs'), join(source, 'main.cjs'))
await writeFile(join(source, 'package.json'), JSON.stringify({ name: 'prime-terminal-native-smoke', version: '1.0.0', main: 'main.cjs' }))
await build({ entryPoints: ['src/main/terminal/manager.ts'], outfile: join(source, 'terminal-manager.cjs'), bundle: true, platform: 'node', format: 'cjs', external: ['node-pty', 'koffi'], logLevel: 'silent' })
const archive = join(directory, 'app.asar')
await createPackageWithOptions(source, archive, { unpackDir: 'node_modules/{node-pty,koffi,@koromix}' })
const environment = { ...process.env, PRIME_TERMINAL_MANAGER: join(archive, 'terminal-manager.cjs') }
delete environment.ELECTRON_RUN_AS_NODE
const result = await promisify(execFile)(electron, [archive], { cwd: process.cwd(), env: environment, windowsHide: true, timeout: 30000 })
process.stdout.write(result.stdout)
process.stderr.write(result.stderr)
console.log(JSON.stringify({ result: 'terminal-asar-ok', archive, unpack: ['node_modules/node-pty/**', 'node_modules/koffi/**', 'node_modules/@koromix/**'] }))

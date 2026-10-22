import { spawnSync } from 'node:child_process'
import { mkdirSync, existsSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const desktopDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const repositoryDir = resolve(desktopDir, '..')
const targetOS = process.env.GOOS || (process.platform === 'win32' ? 'windows' : process.platform === 'darwin' ? 'darwin' : 'linux')
const extension = targetOS === 'windows' ? '.dll' : targetOS === 'darwin' ? '.dylib' : '.so'
const libraryName = `${targetOS === 'windows' ? '' : 'lib'}prime_desktop_engine${extension}`
const outputDir = resolve(desktopDir, 'src-tauri', 'engine')
const environment = { ...process.env, GOOS: targetOS, CGO_ENABLED: '1' }
if (process.env.PRIME_NATIVE_CC) environment.CC = process.env.PRIME_NATIVE_CC
mkdirSync(outputDir, { recursive: true })
const result = spawnSync('go', ['build', '-buildmode=c-shared', '-trimpath', '-ldflags=-s -w', '-o', resolve(outputDir, libraryName), './cmd/desktop-engine'], { cwd: repositoryDir, env: environment, stdio: 'inherit' })
if (result.error) throw result.error
if (result.status !== 0) process.exit(result.status || 1)
if (!existsSync(resolve(outputDir, libraryName))) throw new Error('Native engine library was not produced')
console.log(resolve(outputDir, libraryName))

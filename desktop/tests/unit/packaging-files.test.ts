import { expect, it } from 'vitest'
import { createRequire } from 'node:module'
import { resolve } from 'node:path'

const require = createRequire(import.meta.url)
const { getConfig } = require('app-builder-lib/out/util/config/config')
const { getMainFileMatchers } = require('app-builder-lib/out/fileMatcher')

it.each(['win', 'linux'])('keeps previous releases and setup payloads out of the %s application archive', async (platform) => {
  const root = resolve('.')
  const config = await getConfig(root, 'electron-builder.yml', { directories: { output: 'release/packaging-check' } })
  const output = resolve(root, 'release/packaging-check')
  const matchers = getMainFileMatchers(root, resolve(output, 'resources/app'), (value: string) => value, config[platform], {
    info: { config, projectDir: root, buildResourcesDir: 'build', isPrepackedAppAsar: false, debugLogger: { isEnabled: false } }
  }, output, false)
  const included = (file: string) => matchers.some((matcher: any) => matcher.createFilter()(resolve(root, file), { isDirectory: () => false }))
  expect(included('out/main/index.js')).toBe(true)
  expect(included('out/renderer/index.html')).toBe(true)
  expect(included('package.json')).toBe(true)
  for (const file of ['release/0.4.55/prime-desktop-0.4.55-x64.nsis.7z', 'release/0.4.58/PrimeSetup-0.4.58.exe', 'release/win-unpacked/resources/app.asar', 'installer/payload/PrimeDesktopSetup.exe', '.superpowers/private.png', 'src/main/index.ts']) {
    expect(included(file), file).toBe(false)
  }
})

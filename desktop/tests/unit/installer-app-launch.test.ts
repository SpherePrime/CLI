import { spawn } from 'node:child_process'
import { mkdtemp, readFile, writeFile } from 'node:fs/promises'
import { createRequire } from 'node:module'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, it, vi } from 'vitest'
import { desktopLaunchOptions } from '../../installer/src/main/appLaunch'

it('removes installer AppImage variables and applies the installed application environment', () => {
  vi.stubEnv('APPIMAGE', '/tmp/PrimeSetup.AppImage')
  vi.stubEnv('APPDIR', '/tmp/setup-mount')
  try {
    expect(desktopLaunchOptions().env?.APPIMAGE).toBeUndefined()
    expect(desktopLaunchOptions().env?.APPDIR).toBeUndefined()
    expect(desktopLaunchOptions({ APPIMAGE: '/apps/Prime.AppImage' }).env?.APPIMAGE).toBe(
      '/apps/Prime.AppImage'
    )
  } finally {
    vi.unstubAllEnvs()
  }
})

it.skipIf(process.platform !== 'win32')(
  'opens a visible Electron window after manual installer launch',
  async () => {
    const directory = await mkdtemp(join(tmpdir(), 'prime-installer-launch-'))
    const fixture = join(directory, 'main.cjs')
    const result = join(directory, 'visibility.json')
    await writeFile(
      fixture,
      `
const { app, BrowserWindow } = require('electron')
const { writeFileSync } = require('node:fs')
app.whenReady().then(async () => {
  const window = new BrowserWindow({ width: 400, height: 300, show: true })
  await window.loadURL('data:text/html,<h1>Prime</h1>')
  writeFileSync(process.argv[2], JSON.stringify({ visible: window.isVisible() }))
  app.quit()
})
`
    )
    const options = desktopLaunchOptions()
    delete options.env!.ELECTRON_RUN_AS_NODE
    const executable = createRequire(import.meta.url)('electron') as string
    const child = spawn(executable, [fixture, result], options)
    const code = await new Promise<number | null>((resolve, reject) => {
      child.once('error', reject)
      child.once('exit', resolve)
    })
    expect(code).toBe(0)
    expect(JSON.parse(await readFile(result, 'utf8'))).toEqual({ visible: true })
  }
)

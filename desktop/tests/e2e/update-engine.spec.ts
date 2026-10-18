import { test, expect, _electron as electron } from '@playwright/test'
import { createServer } from 'node:http'
import { createHash } from 'node:crypto'
import { mkdtemp, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

test('the real Electron updater verifies downloads before marking an update ready', async () => {
  test.setTimeout(60000)
  const payload = Buffer.alloc(2 * 1024 * 1024, 31)
  const expectedHash = createHash('sha512').update(payload).digest('base64')
  let corrupt = true
  const server = createServer((request, response) => {
    if (request.url?.startsWith('/latest.yml')) {
      response.end(
        `version: 99.0.0\nfiles:\n  - url: fixture.exe\n    sha512: ${expectedHash}\n    size: ${payload.length}\npath: fixture.exe\nsha512: ${expectedHash}\n`
      )
    } else if (request.url?.startsWith('/fixture.exe')) {
      const bytes = corrupt ? Buffer.alloc(payload.length, 32) : payload
      response.writeHead(200, {
        'Content-Length': bytes.length,
        'Content-Type': 'application/octet-stream'
      })
      response.end(bytes)
    } else {
      response.writeHead(404)
      response.end()
    }
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const port = (server.address() as { port: number }).port
  const profile = await mkdtemp(join(tmpdir(), 'prime-update-engine-'))
  await writeFile(
    join(profile, 'update.yml'),
    'provider: generic\nurl: http://127.0.0.1/\nupdaterCacheDirName: isolated-updates\n'
  )
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    LOCALAPPDATA: profile,
    PRIME_UPDATE_CONFIG: join(profile, 'update.yml'),
    PRIME_UPDATE_URL: `http://127.0.0.1:${port}/`
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({
    args: [
      resolve('tests/fixtures/update-engine-main.cjs'),
      `--user-data-dir=${join(profile, 'user')}`
    ],
    env
  })
  try {
    const page = await desktop.firstWindow()
    await expect(page.getByText('Update engine ready')).toBeVisible()
    const rejected = await desktop.evaluate(async () => (global as any).updateEngine.run())
    expect(rejected.ok).toBe(false)
    expect(rejected.message).toMatch(/sha512|checksum/i)
    expect(rejected.states.some((state: any) => state.state === 'downloaded')).toBe(false)
    corrupt = false
    const accepted = await desktop.evaluate(async () => (global as any).updateEngine.run())
    expect(accepted.ok).toBe(true)
    expect(
      accepted.states.some(
        (state: any) => state.state === 'downloaded' && state.version === '99.0.0'
      )
    ).toBe(true)
  } finally {
    await desktop.close()
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})

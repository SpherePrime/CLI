import { test, expect, _electron as electron } from '@playwright/test'
import { createServer } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

test('checks after startup, animates real progress and offers only restart after download', async () => {
  const server = await createServer({
    configFile: false,
    plugins: [react()],
    server: { host: '127.0.0.1', port: 0, watch: { ignored: ['**/out/**', '**/release/**'] } }
  })
  await server.listen()
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    PRIME_HISTORY_TEST_URL: server.resolvedUrls!.local[0] + 'tests/fixtures/updates.html'
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({ args: [resolve('tests/fixtures/history-main.cjs')], env })
  try {
    const page = await desktop.firstWindow()
    await expect(page.getByText('Workspace', { exact: true })).toBeVisible()
    expect(await page.evaluate(() => (window as any).fixtureCalls)).toEqual([])
    await page.evaluate(() => window.dispatchEvent(new Event('fixture:ready')))
    await expect(page.getByRole('heading', { name: 'Доступна новая версия' })).toBeVisible()
    expect(await page.evaluate(() => (window as any).fixtureCalls)).toEqual(['check'])
    await expect(page.locator('.update-ring-one')).toHaveCSS('animation-name', 'update-orbit')
    await page.evaluate(() =>
      window.dispatchEvent(
        new CustomEvent('fixture:update', {
          detail: {
            state: 'downloading',
            version: '0.4.52',
            percent: 37,
            transferred: 37 * 1024 * 1024,
            total: 100 * 1024 * 1024
          }
        })
      )
    )
    await expect(page.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '37')
    await expect(page.getByText('37.0 / 100.0 MB')).toBeVisible()
    await page.screenshot({ path: 'test-results/update-downloading.png' })
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await expect(page.locator('.update-ring-one')).toHaveCSS('animation-name', 'none')
    await page.evaluate(() => {
      window.dispatchEvent(new CustomEvent('fixture:busy', { detail: true }))
      window.dispatchEvent(
        new CustomEvent('fixture:update', {
          detail: { state: 'downloaded', version: '0.4.52', percent: 100 }
        })
      )
    })
    await expect(
      page.getByRole('heading', { name: 'Необходимо перезапустить приложение' })
    ).toBeVisible()
    await expect(page.getByRole('button', { name: 'Перезапустить', exact: true })).toBeDisabled()
    await expect(page.locator('.update-screen button')).toHaveCount(1)
    await page.evaluate(() =>
      window.dispatchEvent(new CustomEvent('fixture:busy', { detail: false }))
    )
    await page.screenshot({ path: 'test-results/update-ready.png' })
    await page.getByRole('button', { name: 'Перезапустить', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Перезапускаем Prime' })).toBeVisible()
    expect(await page.evaluate(() => (window as any).fixtureCalls)).toEqual(['check', 'install'])
  } finally {
    await desktop.close()
    await server.close()
  }
})

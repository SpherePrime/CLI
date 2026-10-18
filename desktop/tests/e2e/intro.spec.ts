import { test, expect, _electron as electron } from '@playwright/test'
import { createServer } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

test('animates the branded intro, honors reduced motion and releases chat on readiness', async () => {
  const server = await createServer({
    configFile: false,
    plugins: [react()],
    server: { host: '127.0.0.1', port: 0 }
  })
  await server.listen()
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    PRIME_HISTORY_TEST_URL: server.resolvedUrls!.local[0] + 'tests/fixtures/intro.html'
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({ args: [resolve('tests/fixtures/history-main.cjs')], env })
  try {
    const page = await desktop.firstWindow()
    await expect(page.getByRole('status')).toBeVisible()
    await expect(page.locator('.startup-mark rect').first()).toHaveCSS(
      'animation-name',
      'startup-bars'
    )
    await page.screenshot({ path: 'test-results/startup-intro.png' })
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await expect(page.locator('.startup-mark rect').first()).toHaveCSS('animation-name', 'none')
    await page.evaluate(() => window.dispatchEvent(new Event('fixture:ready')))
    await expect(page.getByRole('status')).toHaveCount(0)
    await expect(page.getByText('Chat ready')).toBeVisible()
  } finally {
    await desktop.close()
    await server.close()
  }
})

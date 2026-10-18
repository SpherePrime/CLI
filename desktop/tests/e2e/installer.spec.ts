import { test, expect, _electron as electron } from '@playwright/test'
import { createServer } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

test('the custom installer chooses a folder, presents real stages and opens only after completion', async () => {
  const server = await createServer({
    configFile: false,
    plugins: [react()],
    server: {
      host: '127.0.0.1',
      port: 0,
      watch: {
        ignored: ['**/out/**', '**/release/**', '**/installer/out/**', '**/installer/payload/**']
      }
    }
  })
  await server.listen()
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    PRIME_SETUP_TEST_URL: server.resolvedUrls!.local[0] + 'tests/fixtures/setup.html'
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({ args: [resolve('tests/fixtures/setup-main.cjs')], env })
  try {
    const page = await desktop.firstWindow()
    await expect(
      page.getByRole('heading', { name: 'Добро пожаловать в Prime Desktop.' })
    ).toBeVisible()
    await expect(page.getByText(/CLI|Полная установка/)).toHaveCount(0)
    await page.screenshot({ path: 'test-results/installer-welcome.png' })
    await page.getByRole('button', { name: 'Выбрать папку', exact: true }).click()
    await expect(page.getByRole('textbox', { name: 'Папка установки', exact: true })).toHaveValue(
      'C:\\Apps\\Prime Desktop'
    )
    await page.getByRole('button', { name: 'Установить Prime', exact: true }).click()
    await expect(page.getByRole('status')).toContainText('Установка приложения')
    await expect(
      page.getByRole('button', { name: 'Закрыть установщик', exact: true })
    ).toBeDisabled()
    await page.screenshot({ path: 'test-results/installer-progress.png' })
    await page.evaluate(() =>
      window.dispatchEvent(
        new CustomEvent('fixture:install-status', {
          detail: { phase: 'complete', directory: 'C:\\Apps\\Prime Desktop' }
        })
      )
    )
    await expect(page.getByRole('heading', { name: 'Теперь всё ваше.' })).toBeVisible()
    await page.screenshot({ path: 'test-results/installer-complete.png' })
    await page.getByRole('button', { name: 'Открыть Prime', exact: true }).click()
    expect(await page.evaluate(() => (window as any).fixtureSetupCalls)).toEqual([
      'C:\\Apps\\Prime Desktop',
      'open'
    ])
  } finally {
    await desktop.close()
    await server.close()
  }
})

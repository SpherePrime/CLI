import { test, expect, _electron as electron } from '@playwright/test'
import { createServer } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

test('virtualizes a thousand variable-height messages and preserves both ends of history', async () => {
  const server = await createServer({
    configFile: false,
    plugins: [react()],
    server: { host: '127.0.0.1', port: 0 }
  })
  await server.listen()
  const url = server.resolvedUrls!.local[0] + 'tests/fixtures/history.html'
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    PRIME_HISTORY_TEST_URL: url
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({ args: [resolve('tests/fixtures/history-main.cjs')], env })
  try {
    const page = await desktop.firstWindow()
    await expect(page.getByText('Message 999', { exact: false })).toBeVisible()
    expect(await page.locator('.message').count()).toBeLessThan(40)
    await page.locator('.conversation').evaluate((element) => {
      element.scrollTop = 0
    })
    await expect(page.getByText('Message 0', { exact: false })).toBeVisible()
    await page.getByRole('button', { name: 'Append', exact: true }).click()
    await expect(page.getByText('Message 0', { exact: false })).toBeVisible()
    await page.getByRole('button', { name: 'Новые сообщения' }).click()
    await expect(page.getByText('Newest streamed answer')).toBeVisible()
    expect(await page.locator('.message').count()).toBeLessThan(40)
  } finally {
    await desktop.close()
    await server.close()
  }
})

test('keeps an unresolved approval after a permission-start notification and snapshot refresh', async () => {
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
    PRIME_HISTORY_TEST_URL: server.resolvedUrls!.local[0] + 'tests/fixtures/pending.html'
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({ args: [resolve('tests/fixtures/history-main.cjs')], env })
  try {
    const page = await desktop.firstWindow()
    await expect(page.getByText('Workspace ready')).toBeVisible()
    await page.getByRole('button', { name: 'Request permission', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Разрешить действие?' })).toBeVisible()
    await page.getByRole('button', { name: 'Resolve permission', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Разрешить действие?' })).toHaveCount(0)
  } finally {
    await desktop.close()
    await server.close()
  }
})

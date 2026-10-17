import { test, expect, _electron as electron } from '@playwright/test'
import { createServer } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

test('discards closed models and preserves edits made while a save-and-close is in flight', async () => {
  test.setTimeout(120000)
  const server = await createServer({
    configFile: false,
    plugins: [react()],
    server: { host: '127.0.0.1', port: 0, watch: { ignored: ['**/release/**', '**/out/**'] } }
  })
  await server.listen()
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    PRIME_HISTORY_TEST_URL: server.resolvedUrls!.local[0] + 'tests/fixtures/documents.html'
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({ args: [resolve('tests/fixtures/history-main.cjs')], env })
  try {
    const page = await desktop.firstWindow()
    await page.getByRole('button', { name: 'Open A', exact: true }).click()
    const code = page.getByRole('textbox', { name: 'Код файла', exact: true })
    await expect(code).toBeAttached({ timeout: 30000 })
    await code.press('ControlOrMeta+A')
    await page.keyboard.insertText('discardedDraft')
    await page.getByRole('button', { name: 'Open B', exact: true }).click()
    await page.getByRole('button', { name: 'Закрыть файл A.ts', exact: true }).click()
    await page.getByRole('button', { name: 'Не сохранять', exact: true }).click()
    await page.getByRole('button', { name: 'Open A', exact: true }).click()
    await expect(page.locator('.view-lines')).toContainText('original')
    await expect(page.locator('.view-lines')).not.toContainText('discardedDraft')
    await code.press('ControlOrMeta+A')
    await page.keyboard.insertText('firstSave')
    await page.getByRole('button', { name: 'Закрыть файл A.ts', exact: true }).click()
    await page.getByRole('button', { name: 'Сохранить', exact: true }).last().click()
    await page.getByRole('button', { name: 'Отмена', exact: true }).click()
    await code.press('ControlOrMeta+A')
    await page.keyboard.insertText('newerUnsavedEdit')
    await page.evaluate(() => window.dispatchEvent(new Event('fixture:complete-save')))
    await expect(page.getByRole('tab', { name: 'A.ts', exact: false })).toBeVisible()
    await expect(page.locator('.view-lines')).toContainText('newerUnsavedEdit')
    await expect(page.getByText('Не сохранено', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Закрыть файл A.ts', exact: true }).click()
    await page.getByRole('button', { name: 'Не сохранять', exact: true }).click()
    await expect(page.locator('p[role=alert]')).toHaveCount(0)
  } finally {
    await desktop.close()
    await server.close()
  }
})

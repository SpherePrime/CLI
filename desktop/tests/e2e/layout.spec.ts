import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

const sizes = [
  [1440, 900],
  [1100, 760],
  [820, 700],
  [560, 720]
] as const

test('layout adapts to window width without overflow', async () => {
  test.setTimeout(180000)
  const { app, page, provider } = await launchWorkbench()
  const prefix = process.env.LAYOUT_SHOT_PREFIX || 'layout'
  try {
    await app.evaluate(({ BrowserWindow }) =>
      BrowserWindow.getAllWindows()[0].setMinimumSize(420, 420)
    )
    await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('Проверка раскладки')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(page.locator('.message.assistant').first()).toBeVisible({ timeout: 30000 })
    for (const [width, height] of sizes) {
      await app.evaluate(
        ({ BrowserWindow }, size) => BrowserWindow.getAllWindows()[0].setSize(size[0], size[1]),
        [width, height]
      )
      await page.waitForTimeout(500)
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth - document.documentElement.clientWidth
      )
      expect(overflow).toBeLessThanOrEqual(0)
      await expect(page.locator('.composer')).toBeVisible()
      await page.screenshot({ path: `test-results/${prefix}-${width}.png` })
    }
    await app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].setSize(1440, 900))
    await page.waitForTimeout(400)
    const files = page.locator('.inspector-rail .inspector-tabs button[aria-label="Файлы"]')
    await files.click()
    await expect(page.locator('.app-shell > .inspector-panel')).toBeVisible()
    await expect(page.locator('.inspector-rail')).toBeVisible()
    await page.screenshot({ path: `test-results/${prefix}-inspector-open.png` })
    await files.click()
    await expect(page.locator('.app-shell > .inspector-panel')).toHaveCount(0)
    await page.locator('.workspace-topbar').getByRole('button', { name: 'Панель проектов', exact: true }).click()
    await expect(page.locator('.app-shell > .sidebar')).toHaveCount(0)
    await expect(page.locator('.project-rail')).toBeVisible()
    await page.screenshot({ path: `test-results/${prefix}-collapsed.png` })
    await page.locator('.project-rail > button').first().click()
    await expect(page.locator('.app-shell > .sidebar')).toBeVisible()
    await page.evaluate(() => window.prime.preferences({ theme: 'light' }))
    await page.reload()
    await page.locator('.composer').waitFor()
    await page.waitForTimeout(500)
    await page.screenshot({ path: `test-results/${prefix}-light.png` })
  } finally {
    await app.close()
    await provider.close()
  }
})

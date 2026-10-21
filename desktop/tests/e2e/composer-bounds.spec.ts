import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('composer keeps send inside its border before interaction and after resizing', async () => {
  const fixture = await launchWorkbench()
  const { app, page } = fixture
  try {
    await app.evaluate(({ BrowserWindow }) => {
      BrowserWindow.getAllWindows()[0].setMinimumSize(420, 420)
    })
    await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('Layout check')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(page.locator('.message.assistant').first()).toBeVisible()
    for (const width of [1440, 1100, 960, 850, 740, 620, 560, 460]) {
      await app.evaluate(({ BrowserWindow }, width) => {
        BrowserWindow.getAllWindows()[0].setSize(width, 800)
      }, width)
      await page.reload()
      await page.locator('.composer .send').waitFor()
      await page.locator('.model-control-name').evaluate((element) => {
        element.textContent = 'Very long provider model name with reasoning support'
      })
      await expect
        .poll(
          async () =>
            page.locator('.composer').evaluate((composer) => {
              const border = composer.getBoundingClientRect()
              const send = composer.querySelector('.send')!.getBoundingClientRect()
              return send.right <= border.right - 4 && send.left >= border.left + 4
            }),
          { message: `send remains inside composer at ${width}px` }
        )
        .toBe(true)
    }
    await page.screenshot({ path: 'test-results/composer-bounds.png' })
  } finally {
    await fixture.close()
  }
})

import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('copy button sits next to the user bubble', async () => {
  test.setTimeout(120000)
  const workbench = await launchWorkbench()
  const { page } = workbench
  try {
    await workbench.app.evaluate(({ BrowserWindow }) =>
      BrowserWindow.getAllWindows()[0].setSize(1200, 720)
    )
    const textbox = page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })
    await textbox.fill('проверка кнопки копирования')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    const bubble = page.locator('.message.user .message-body').last()
    await expect(bubble).toContainText('проверка кнопки копирования', { timeout: 15000 })
    await page.locator('.message.user').last().hover()
    const button = page.locator('.message.user .copy-message').last()
    await expect(button).toBeVisible()
    const bubbleBox = await bubble.boundingBox()
    const buttonBox = await button.boundingBox()
    const articleBox = await page.locator('.message.user').last().boundingBox()
    console.log('bubble', JSON.stringify(bubbleBox))
    console.log('button', JSON.stringify(buttonBox))
    console.log('article', JSON.stringify(articleBox))
    expect(buttonBox).not.toBeNull()
    expect(bubbleBox).not.toBeNull()
    expect(articleBox).not.toBeNull()
    expect(buttonBox!.x + buttonBox!.width).toBeLessThanOrEqual(articleBox!.x + articleBox!.width)
    expect(buttonBox!.x).toBeGreaterThanOrEqual(bubbleBox!.x + bubbleBox!.width - 2)
    expect(buttonBox!.y + buttonBox!.height).toBeLessThanOrEqual(
      bubbleBox!.y + bubbleBox!.height + 2
    )
    await page.screenshot({ path: 'test-results/copy-button.png' })
  } finally {
    await workbench.close()
  }
})

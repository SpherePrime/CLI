import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('compact rails keep tools reachable and dock tab changes preserve height', async () => {
  test.setTimeout(90000)
  const { app, page, provider } = await launchWorkbench()
  try {
    await app.evaluate(({ BrowserWindow }) => {
      const window = BrowserWindow.getAllWindows()[0]
      window.setMinimumSize(420, 420)
      window.setSize(620, 760)
    })
    await expect(page.locator('.project-rail')).toBeVisible()
    await expect(page.locator('.inspector-rail')).toBeVisible()
    await page.locator('.project-rail > button').first().click()
    await expect(page.locator('.project-flyout .sidebar')).toBeVisible()
    await page.keyboard.press('Escape')
    await page.locator('.inspector-tabs button[aria-label="Файлы"]').click()
    await expect(page.locator('.inspector-content')).toBeVisible()
    await page.keyboard.press('Escape')
    await page.keyboard.press('Control+`')
    const dock = page.locator('.workbench-dock')
    await expect(dock).toBeVisible()
    const height = await dock.evaluate((element) => element.getBoundingClientRect().height)
    for (const name of ['Запуск', 'Автоматизации', 'Команды', 'GitHub', 'Окна', 'Терминал']) {
      await page.getByRole('tab', { name, exact: true }).click()
      await expect(dock).not.toHaveClass(/expanded/)
      expect(
        Math.abs(
          (await dock.evaluate((element) => element.getBoundingClientRect().height)) - height
        )
      ).toBeLessThan(3)
    }
    await page.getByRole('button', { name: 'Развернуть панель', exact: true }).click()
    await page.getByRole('tab', { name: 'Запуск', exact: true }).click()
    await expect(dock).toHaveClass(/expanded/)
    await page.getByRole('button', { name: 'Свернуть панель', exact: true }).click()
    expect(
      Math.abs((await dock.evaluate((element) => element.getBoundingClientRect().height)) - height)
    ).toBeLessThan(3)
    await page.screenshot({ path: 'test-results/compact-workbench.png' })
    await page.getByRole('button', { name: 'Скрыть инструменты', exact: true }).click()
    await app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].setSize(1440, 900))
    const separator = page.getByRole('separator', { name: 'Ширина рабочей области', exact: true })
    await separator.focus()
    await page.keyboard.press('End')
    await expect
      .poll(() =>
        page.locator('.inspector').evaluate((element) => element.getBoundingClientRect().width)
      )
      .toBeGreaterThan(1000)
    await expect
      .poll(() => page.evaluate(async () => (await window.prime.preferences()).inspectorWidth))
      .toBeGreaterThan(1000)
  } finally {
    await app.close()
    await provider.close()
  }
})

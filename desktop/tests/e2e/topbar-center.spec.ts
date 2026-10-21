import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('command search stays centered in the window with a long chat title', async () => {
  const fixture = await launchWorkbench()
  const { page, app, project } = fixture
  try {
    await page.evaluate(async (id) => {
      const session = await window.prime.call(id, 'createSession', {
        title: 'Очень длинное название проекта и задачи для проверки расположения поиска'
      })
      await window.prime.call(id, 'currentSession', { sessionId: session.id })
    }, project.id)
    await page.reload()
    for (const width of [1440, 1100, 980]) {
      await app.evaluate(
        ({ BrowserWindow }, width) => BrowserWindow.getAllWindows()[0].setSize(width, 800),
        width
      )
      await expect
        .poll(() =>
          page.locator('.topbar-search').evaluate((button) => {
            const bounds = button.getBoundingClientRect()
            return Math.abs(bounds.left + bounds.width / 2 - window.innerWidth / 2)
          })
        )
        .toBeLessThan(1)
      const overlap = await page.locator('.topbar-search').evaluate((button) => {
        const bounds = button.getBoundingClientRect()
        const actions = document
          .querySelector('.workspace-topbar .header-actions')!
          .getBoundingClientRect()
        const crumbs = document
          .querySelector('.workspace-topbar .breadcrumbs')!
          .getBoundingClientRect()
        return crumbs.right > bounds.left || actions.left < bounds.right
      })
      expect(overlap).toBe(false)
    }
    await page.screenshot({ path: 'test-results/topbar-centered.png' })
  } finally {
    await fixture.close()
  }
})

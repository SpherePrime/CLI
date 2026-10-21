import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('working duration survives chat switching and finished durations stay in history', async () => {
  const fixture = await launchWorkbench()
  const { page, project } = fixture
  const seconds = (text: string) =>
    text.split(':').reduce((total, value) => total * 60 + Number(value), 0)
  try {
    const session = await page.evaluate(
      (id) => window.prime.call(id, 'createSession', { title: 'Timed task' }),
      project.id
    )
    await page.reload()
    if (!(await page.locator('.app-shell > .sidebar').isVisible()))
      await page.locator('.workspace-topbar [data-projects-toggle]').click()
    const row = page.locator('.project-group .session-link').filter({ hasText: 'Timed task' })
    await row.click()
    await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('HOLD_FIXTURE')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect
      .poll(async () => seconds(await page.locator('.working-time').innerText()))
      .toBeGreaterThanOrEqual(3)
    const before = seconds(await page.locator('.working-time').innerText())
    await page.locator('.sidebar-actions .new-chat').click()
    await expect(page.locator('.working-time')).toHaveCount(0)
    await row.click()
    await expect
      .poll(async () => seconds(await page.locator('.working-time').innerText()))
      .toBeGreaterThanOrEqual(before)
    await page.evaluate(({ id, sessionId }) => window.prime.call(id, 'cancel', { sessionId }), {
      id: project.id,
      sessionId: session.id
    })
    await expect(page.locator('.completed-working')).toContainText('Работал')
    await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('Reply shortly')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(page.locator('.completed-working')).toHaveCount(2)
    const summaries = await page.locator('.completed-working').allTextContents()
    await page.reload()
    await expect(page.locator('.completed-working')).toHaveText(summaries)
  } finally {
    await fixture.close()
  }
})

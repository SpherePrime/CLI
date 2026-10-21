import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('new chat hides legacy placeholders and receives a generated task title', async () => {
  const fixture = await launchWorkbench()
  const { page, project } = fixture
  try {
    await page.locator('.sidebar-actions .new-chat').click()
    await expect(page.locator('.workspace-topbar .breadcrumbs strong')).toHaveText('Новый чат')
    await page.evaluate(async (id) => {
      const sessions = await window.prime.call(id, 'sessions')
      await window.prime.call(id, 'renameSession', {
        sessionId: sessions[0].id,
        session: { ...sessions[0], title: 'Untitled Session' }
      })
    }, project.id)
    await page.reload()
    await expect(page.locator('.workspace-topbar .breadcrumbs strong')).toHaveText('Новый чат')
    await expect(page.getByText('Untitled Session', { exact: true })).toHaveCount(0)
    await page
      .getByRole('textbox', { name: 'Сообщение Prime' })
      .fill('Проверь генерацию названия чата')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(page.locator('.workspace-topbar .breadcrumbs strong')).toHaveText(
      'Проверка названия чата'
    )
    await page.reload()
    await expect(page.locator('.workspace-topbar .breadcrumbs strong')).toHaveText(
      'Проверка названия чата'
    )
  } finally {
    await fixture.close()
  }
})

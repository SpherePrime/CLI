import { test, expect, type Page } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

async function sidebarState(page: Page) {
  return page.evaluate(() => {
    const rows = [...document.querySelectorAll('.project-group .session-row')]
    const recent = [...document.querySelectorAll('.recent-chat')]
    return {
      rows: rows.length,
      active: rows.filter((row) => row.classList.contains('active')).length,
      recentActive: recent.filter((row) => row.classList.contains('active')).length
    }
  })
}

test('creating many chats keeps one selection and every chat can be deleted', async () => {
  test.setTimeout(240000)
  const workbench = await launchWorkbench()
  const page = workbench.page
  try {
    await page.locator('.project-item').filter({ hasText: 'Личные чаты' }).click()
    await expect(page.locator('.workspace-topbar')).toContainText('Личные чаты')
    const newChat = page.locator('.new-chat')
    for (let index = 0; index < 4; index++) await newChat.click()
    await Promise.all(Array.from({ length: 4 }, () => newChat.click()))
    await page.keyboard.press('Control+N')
    await page.keyboard.press('Control+N')
    await page.waitForTimeout(2000)
    const personal = await page.evaluate(async () =>
      (await window.prime.projects()).find((item) => item.kind === 'chat')!
    )
    const sessions = await page.evaluate(
      (id) => window.prime.call(id, 'sessions'),
      personal.id
    )
    const state = await sidebarState(page)
    await page.screenshot({ path: 'test-results/chat-sidebar-many.png' })
    expect(state.rows).toBe(sessions.length)
    expect(state.active).toBe(1)
    expect(state.recentActive).toBe(0)

    const group = page.locator('.project-group').filter({ hasText: 'Личные чаты' })
    for (let remaining = sessions.length; remaining > 0; remaining--) {
      const row = group.locator('.session-row').first()
      await row.hover()
      await row.getByRole('button', { name: 'Удалить чат', exact: true }).click()
      await expect(group.locator('.session-row')).toHaveCount(remaining - 1)
    }
    await expect(page.locator('.recent-chat')).toHaveCount(0)
    expect(
      await page.evaluate((id) => window.prime.call(id, 'sessions'), personal.id)
    ).toHaveLength(0)
    await page.screenshot({ path: 'test-results/chat-sidebar-empty.png' })
    expect(workbench.errors).toEqual([])
  } finally {
    await workbench.close()
  }
})

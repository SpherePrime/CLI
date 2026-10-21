import { test, expect } from '@playwright/test'
import { DatabaseSync } from 'node:sqlite'
import { join } from 'node:path'
import { launchWorkbench } from '../fixtures/workbench-app'

test('history pages from disk, searches older messages and exports the complete chat', async () => {
  test.setTimeout(180000)
  const fixture = await launchWorkbench()
  const { page, project, root, app } = fixture
  try {
    const session = await page.evaluate(
      (id) => window.prime.call(id, 'createSession', { title: 'Paged history' }),
      project.id
    )
    await page.evaluate(
      (id) => window.prime.call(id, 'createSession', { title: 'Switch target' }),
      project.id
    )
    const database = new DatabaseSync(join(root, '.prime', 'prime.db'))
    try {
      const insert = database.prepare(
        'INSERT INTO messages(id,session_id,role,parts,created_at,updated_at,finished_at) VALUES(?,?,?,?,?,?,?)'
      )
      const start = Math.floor(Date.now() / 1000) - 1000
      for (let index = 0; index < 360; index++) {
        const text = `Disk history ${String(index).padStart(3, '0')}`
        insert.run(`disk-${String(index).padStart(3, '0')}`, session.id, index % 2 ? 'assistant' : 'user',
          JSON.stringify([{ type: 'text', data: { text } },
            ...(index % 2 ? [{ type: 'finish', data: { reason: 'end_turn', time: start + index } }] : [])]),
          start + index, start + index, start + index)
      }
    } finally {
      database.close()
    }
    if (!(await page.locator('.app-shell > .sidebar').isVisible()))
      await page.locator('.workspace-topbar [data-projects-toggle]').click()
    const pagedLink = page.locator('.project-group .session-link').filter({ hasText: 'Paged history' })
    for (let attempt = 0; attempt < 9 && (await page.locator('.project-group .session-link').count()) === 0; attempt++)
      await page.waitForTimeout(2000)
    if ((await page.locator('.project-group .session-link').count()) === 0)
      await page.locator('.project-group .project-item').first().click()
    await pagedLink.waitFor({ state: 'visible', timeout: 45000 })
    await pagedLink.click()
    await expect(page.locator('[data-message-id="disk-359"]')).toBeVisible()
    const initial = await page.evaluate(({ id, sessionId }) =>
      window.prime.call(id, 'messagePage', { sessionId, limit: 60 }),
      { id: project.id, sessionId: session.id })
    expect(initial.messages).toHaveLength(60)
    expect(initial.messages[0].id).toBe('disk-300')
    const view = page.locator('.conversation')
    await view.hover()
    await page.mouse.wheel(0, -50000)
    await expect(page.locator('[data-message-id="disk-299"]')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Предыдущие сообщения', exact: true })).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Следующие сообщения', exact: true })).toHaveCount(0)
    for (let attempt = 0; attempt < 10; attempt++) {
      await view.hover()
      await page.mouse.wheel(0, -50000)
      await page.waitForTimeout(250)
    }
    await expect(page.getByRole('button', { name: 'Новые сообщения', exact: true })).toBeVisible()
    await page.keyboard.press('ControlOrMeta+f')
    await page.getByRole('textbox', { name: 'Поиск в чате', exact: true }).fill('Disk history 000')
    await expect(page.locator('[data-message-id="disk-000"]')).toBeVisible()
    await page.getByRole('textbox', { name: 'Поиск в чате', exact: true }).fill('Disk history 010')
    await expect(page.locator('[data-message-id="disk-010"]')).toBeVisible()
    await page.getByRole('textbox', { name: 'Поиск в чате', exact: true }).fill('Disk history 350')
    await expect(page.locator('[data-message-id="disk-350"]')).toBeVisible()
    await page.getByRole('button', { name: 'Действия чата', exact: true }).click()
    await page.getByRole('button', { name: 'Скопировать чат', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Скопировано', exact: true })).toBeVisible()
    const copied = await app.evaluate(({ clipboard }) => clipboard.readText())
    expect(copied).toContain('Disk history 000')
    expect(copied).toContain('Disk history 359')
    expect(copied.indexOf('Disk history 000')).toBeLessThan(copied.indexOf('Disk history 359'))
    await page.keyboard.press('Escape')
    await page.locator('.project-group .session-link').filter({ hasText: 'Switch target' }).click()
    await expect(page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })).toBeVisible()
    await pagedLink.click()
    await expect(page.locator('[data-message-id="disk-359"]')).toBeVisible()
  } finally {
    await fixture.close()
  }
})

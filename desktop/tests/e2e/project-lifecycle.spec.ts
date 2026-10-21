import { test, expect } from '@playwright/test'
import { readFile } from 'node:fs/promises'
import { launchWorkbench } from '../fixtures/workbench-app'

test('new chat belongs to the selected project and removing it persists without deleting files', async () => {
  const fixture = await launchWorkbench()
  const { page, project, root } = fixture
  try {
    const before = await page.evaluate((id) => window.prime.call(id, 'sessions'), project.id)
    await page.locator('.sidebar-actions .new-chat').click()
    await expect
      .poll(async () =>
        page.evaluate(async (id) => {
          const sessions = await window.prime.call(id, 'sessions')
          return sessions.length
        }, project.id)
      )
      .toBe(before.length + 1)
    const created = await page.evaluate((id) => window.prime.call(id, 'sessions'), project.id)
    const close = page.getByRole('button', { name: `Закрыть проект ${root}`, exact: true })
    await close.click()
    await expect(close).toHaveCount(0)
    expect(
      await page.evaluate(
        async (root) => (await window.prime.preferences()).recent.includes(root),
        root
      )
    ).toBe(false)
    await page.reload()
    await expect(page.locator('.composer')).toBeVisible()
    expect(
      await page.evaluate(
        async (root) => (await window.prime.preferences()).recent.includes(root),
        root
      )
    ).toBe(false)
    expect(await readFile(`${root}/app.ts`, 'utf8')).toContain('export const version = 1')
    const restored = await page.evaluate(async (root) => {
      const project = await window.prime.openProject(root)
      return window.prime.call(project!.id, 'sessions')
    }, root)
    expect(restored.map((session: { id: string }) => session.id).sort()).toEqual(
      created.map((session: { id: string }) => session.id).sort()
    )
  } finally {
    await fixture.close()
  }
})

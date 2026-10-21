import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('access mode switches reliably and stays isolated per chat across restart', async () => {
  const fixture = await launchWorkbench()
  let page = fixture.page
  try {
    const [first, second] = await page.evaluate(async (id) => {
      const first = await window.prime.call(id, 'createSession', { title: 'Access A' })
      const second = await window.prime.call(id, 'createSession', { title: 'Access B' })
      await window.prime.call(id, 'currentSession', { sessionId: first.id })
      return [first, second]
    }, fixture.project.id)
    await page.reload()
    const trigger = () => page.getByRole('button', { name: 'Режим доступа', exact: true })
    const choose = async (full: boolean) => {
      await trigger().click()
      await page
        .getByRole('menuitemradio', { name: full ? /^Полный доступ/ : /^С подтверждением/ })
        .click()
      await expect(trigger()).toContainText(full ? 'Полный доступ' : 'С подтверждением')
      await expect(trigger()).toBeEnabled()
    }
    await choose(true)
    await expect
      .poll(() =>
        page.evaluate(
          async (id) => (await window.prime.preferences()).skipBySession?.[id],
          first.id
        )
      )
      .toBe(true)
    await choose(false)
    await choose(true)
    if (!(await page.locator('.app-shell > .sidebar').isVisible()))
      await page.locator('.workspace-topbar [data-projects-toggle]').click()
    await page.locator('.session-link').filter({ hasText: 'Access B' }).first().click()
    await expect(trigger()).toContainText('С подтверждением')
    await choose(false)
    await page.locator('.session-link').filter({ hasText: 'Access A' }).first().click()
    await expect(trigger()).toContainText('Полный доступ')
    ;({ page } = await fixture.restart())
    await expect(trigger()).toContainText('Полный доступ')
    await page.locator('.session-link').filter({ hasText: 'Access B' }).first().click()
    await expect(trigger()).toContainText('С подтверждением')
    const restoredProjectId = await page.evaluate(
      async (path) => (await window.prime.projects()).find((project) => project.path === path)!.id,
      fixture.root
    )
    const modes = await page.evaluate(
      async ({ id, first, second }) => [
        await window.prime.call(id, 'getSkip', { sessionId: first }),
        await window.prime.call(id, 'getSkip', { sessionId: second })
      ],
      { id: restoredProjectId, first: first.id, second: second.id }
    )
    expect(modes.map((mode) => mode.skip)).toEqual([true, false])
    await page.locator('.sidebar-actions .new-chat').click()
    await expect(trigger()).toContainText('С подтверждением')
    await page.locator('.session-link').filter({ hasText: 'Access A' }).first().click()
    await expect(trigger()).toContainText('Полный доступ')
  } finally {
    await fixture.close()
  }
})

import { test, expect } from '@playwright/test'
import { mkdir } from 'node:fs/promises'
import { join } from 'node:path'
import { launchWorkbench } from '../fixtures/workbench-app'

test('project cards keep chats grouped, search across chats and create them in the right project', async () => {
  const fixture = await launchWorkbench()
  const { page, root, app } = fixture
  try {
    const paths = [join(root, 'Website'), join(root, 'API')]
    for (const path of paths) await mkdir(join(path, '.prime'), { recursive: true })
    for (const path of paths) {
      await app.evaluate(({ dialog }, path) => {
        dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [path] })
      }, path)
      await page.evaluate(async () => {
        const project = await window.prime.openProject()
        if (!project) throw new Error('Project was not opened')
        for (const title of ['Landing page', 'Fix navigation', 'Review changes'])
          await window.prime.call(project.id, 'createSession', { title })
      })
    }
    await page.evaluate(async () => {
      const personal = await window.prime.openChat()
      await window.prime.call(personal.id, 'createSession', { title: 'Weekend ideas' })
    })
    await page.reload()
    if (!(await page.locator('.app-shell > .sidebar').isVisible()))
      await page.locator('.workspace-topbar [data-projects-toggle]').click()
    const website = page.getByRole('group', { name: 'Проект Website', exact: true })
    const api = page.getByRole('group', { name: 'Проект API', exact: true })
    await website.locator('.project-item').click()
    await api.locator('.project-item').click()
    await expect(website.locator('.project-chat-list .session-row')).toHaveCount(3)
    await expect(website.locator('.project-chevron')).toHaveClass(/expanded/)
    await expect(page.locator('.recent-chat')).toHaveCount(1)
    await expect(page.locator('.recent-chat')).toContainText('Weekend ideas')
    const search = page.getByRole('textbox', { name: 'Поиск проектов и чатов' })
    await search.fill('Fix navigation')
    await expect(website.locator('.project-chat-list .session-row')).toHaveCount(1)
    await expect(api.locator('.project-chat-list .session-row')).toHaveCount(1)
    await search.fill('')
    await website.getByRole('button', { name: 'Новый чат в проекте Website', exact: true }).click()
    await expect(website.locator('.session-row.active')).toContainText('Новый чат')
    await expect(website.locator('.project-count')).toHaveText('4')
    await page.screenshot({ path: 'test-results/project-navigation.png' })
    await page.locator('.left-resizer').focus()
    await page.keyboard.press('Home')
    await page.keyboard.press('ArrowRight')
    await expect(website).toBeVisible()
    const overflow = await website.evaluate((element) => element.scrollWidth - element.clientWidth)
    expect(overflow).toBeLessThanOrEqual(1)
    await page.screenshot({ path: 'test-results/project-navigation-narrow.png' })
    await page.evaluate(() => window.prime.preferences({ theme: 'light' }))
    await page.reload()
    await page.locator('.composer').waitFor()
    if (!(await page.locator('.app-shell > .sidebar').isVisible()))
      await page.locator('.workspace-topbar [data-projects-toggle]').click()
    await expect(website).toBeVisible()
    await page.screenshot({ path: 'test-results/project-navigation-light.png' })
  } finally {
    await fixture.close()
  }
})

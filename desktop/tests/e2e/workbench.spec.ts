import { test, expect } from '@playwright/test'
import { readFile, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { launchWorkbench } from '../fixtures/workbench-app'

test('workspace actions, working context and appearance persist in real Electron', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  const { page, root, app, provider, errors } = fixture
  try {
    await page
      .locator('.welcome')
      .evaluate((element) =>
        Promise.all(element.getAnimations({ subtree: true }).map((animation) => animation.finished))
      )
    await page.screenshot({ path: 'test-results/workbench-welcome-dark.png' })
    await page.keyboard.press('Control+k')
    await expect(page.getByRole('dialog', { name: 'Быстрые действия' })).toBeVisible()
    await page.getByRole('combobox', { name: 'Найти действие или чат' }).fill('Режим фокуса')
    await page.keyboard.press('Enter')
    await expect(page.locator('.app-shell')).toHaveClass(/focus-mode/)
    await expect(page.locator('.sidebar')).toHaveCount(0)
    await page.keyboard.press('Control+Shift+f')
    await expect(page.locator('.sidebar')).toBeVisible()

    await expect(page.getByRole('button', { name: 'Задача и контекст', exact: true })).toHaveCount(
      0
    )
    await page.evaluate(async (root) => {
      await window.prime.preferences({
        agentContexts: {
          [root]: {
            enabled: true,
            goal: 'Fix login',
            constraints: 'Keep public exports',
            criteria: 'Existing users can sign in',
            sources: [{ id: 'file:app.ts', kind: 'file', path: 'app.ts' }]
          }
        }
      })
    }, root)
    await writeFile(join(root, 'app.ts'), 'export const version = 2\n')
    await page.getByRole('textbox', { name: 'Сообщение Prime', exact: true }).fill('Fix login now')
    await expect(page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })).toHaveValue(
      'Fix login now'
    )
    try {
      await page
        .getByRole('button', { name: 'Отправить сообщение', exact: true })
        .click({ timeout: 10000 })
    } catch (error) {
      console.log(
        'WORKSPACE_STATE',
        await page.evaluate(async () => ({
          info: await window.prime.info(),
          projects: await window.prime.projects(),
          draft: (document.querySelector('textarea') as HTMLTextAreaElement)?.value,
          errors: [...document.querySelectorAll('[role=alert]')].map((item) => item.textContent)
        }))
      )
      await page.screenshot({ path: 'test-results/workbench-send-failed.png' })
      throw error
    }
    await expect(page.locator('.message.assistant .markdown')).toContainText(
      'Соединение работает',
      { timeout: 30000 }
    )
    expect(
      provider.prompts.some((prompt) => JSON.stringify(prompt).includes('export const version = 2'))
    ).toBe(true)
    expect(
      provider.prompts.some((prompt) =>
        JSON.stringify(prompt).includes('Existing users can sign in')
      )
    ).toBe(true)
    await expect(page.locator('.message.user')).not.toContainText('working-context')
    await page.keyboard.press('Control+f')
    await page.getByRole('textbox', { name: 'Поиск в чате', exact: true }).fill('Соединение')
    await expect(page.locator('.conversation-search [role=status]')).toHaveText('1 / 1')
    await expect(page.locator('.message.search-match')).toContainText('Соединение работает')
    await page.keyboard.press('Escape')
    await expect(page.locator('.conversation-search')).toHaveCount(0)

    const downloadPath = join(root, 'export.md')
    await app.evaluate(
      ({ session }, path) =>
        session.defaultSession.once('will-download', (_event, item) => item.setSavePath(path)),
      downloadPath
    )
    await page.getByRole('button', { name: 'Действия чата' }).click()
    await page.getByRole('button', { name: 'Экспорт в Markdown' }).click()
    await expect
      .poll(async () => readFile(downloadPath, 'utf8').catch(() => ''))
      .toContain('Соединение работает')

    await page.getByRole('button', { name: 'app.ts', exact: true }).click()
    const editor = page.getByRole('textbox', { name: 'Код файла', exact: true })
    await editor.press('Control+a')
    await page.keyboard.insertText('unsavedDraft')
    await expect(page.locator('.file-editor .view-lines')).toContainText('unsavedDraft')
    await page.keyboard.press('Control+Shift+f')
    await expect(page.locator('.file-editor')).toBeHidden()
    await page.keyboard.press('Control+Shift+f')
    await editor.press('Control+z')
    await expect(page.locator('.file-editor .view-lines')).not.toContainText('unsavedDraft')
    await page.getByRole('button', { name: 'Закрыть файл app.ts', exact: true }).click()

    await page.keyboard.press('Control+,')
    await page.getByRole('button', { name: 'Компактная', exact: true }).click()
    await page.getByRole('checkbox', { name: 'Меньше анимаций' }).check()
    await page.getByRole('button', { name: 'Светлая', exact: false }).click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
    await page.screenshot({ path: 'test-results/workbench-settings-light.png' })
    await page.getByRole('button', { name: 'Закрыть', exact: true }).click()
    await page.reload()
    await expect(page.locator('html')).toHaveAttribute('data-density', 'compact')
    await expect(page.locator('html')).toHaveAttribute('data-motion', 'reduced')
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
    await expect(page.locator('.context-control')).toHaveCount(0)
    await page.screenshot({ path: 'test-results/workbench-chat-light.png' })
    expect(errors).toEqual([])
  } finally {
    await fixture.close()
  }
})

import { expect } from '@playwright/test'
import { readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { startUiAudit } from './native-ui-runner.mjs'
import { auditProject } from './native-ui-project.mjs'

const audit = await startUiAudit()
const control = page => page.getByRole('button', { name: 'Модель и уровень размышлений', exact: true })
async function openModels(page) {
  await control(page).click()
  await page.locator('.reasoning-popover-model').click()
  await page.getByRole('listbox', { name: 'Модели' }).waitFor()
}
async function send(page, text, model) {
  const before = audit.provider.requests.length
  const replies = await page.locator('article.message.assistant').count()
  await page.getByRole('textbox', { name: 'Сообщение Prime', exact: true }).fill(text)
  await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
  await expect.poll(() => audit.provider.requests.slice(before).filter(request => request.stream).map(request => request.model)).toContain(model)
  await expect(page.getByRole('button', { name: 'Остановить задачу', exact: true })).toHaveCount(0, { timeout: 30000 })
  await expect.poll(() => page.locator('article.message.assistant').count()).toBeGreaterThan(replies)
  await expect(page.locator('article.message.assistant').last()).toContainText(/UI_CHAT_[AB] response/)
  await expect(page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })).toHaveValue('')
}
async function settings(page, tab) {
  await page.getByRole('button', { name: 'Настройки', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Настройки', exact: true })
  await dialog.getByRole('button', { name: tab, exact: true }).click()
  return dialog
}
async function closeSettings(page) {
  await page.getByRole('dialog', { name: 'Настройки', exact: true }).getByRole('button', { name: 'Закрыть', exact: true }).click()
}
try {
  await audit.launch()
  await audit.step('01-startup', async page => { await expect(control(page)).toContainText(/audit[- ]alpha/i) })
  await audit.step('02-first-chat-alpha', page => send(page, 'UI_CHAT_A initial', 'audit-alpha'))
  await audit.step('03-chat-model-beta', async page => {
    await openModels(page)
    await page.getByRole('option', { name: /Audit beta/ }).first().click()
    await expect(control(page)).toContainText(/audit[- ]beta/i)
    await send(page, 'UI_CHAT_A selected beta', 'audit-beta')
  })
  await audit.step('04-reasoning', async page => {
    await control(page).click()
    const slider = page.getByRole('slider', { name: 'Уровень размышлений' })
    await slider.focus()
    await slider.press('End')
    await expect(slider).toHaveAttribute('aria-valuetext', /Высок|Максим/)
    await page.keyboard.press('Escape')
  })
  await audit.step('05-favorites', async page => {
    await openModels(page)
    await page.getByRole('button', { name: 'Добавить в избранное: Audit beta', exact: true }).first().click()
    await expect(page.getByRole('button', { name: 'Удалить из избранного: Audit beta', exact: true }).first()).toHaveAttribute('aria-pressed', 'true')
    await page.keyboard.press('Escape')
  })
  await audit.step('05b-access-per-chat', async page => {
    await page.getByRole('button', { name: 'Режим доступа', exact: true }).click()
    await page.getByRole('menuitemradio', { name: /Полный доступ/ }).click()
    await expect(page.getByRole('button', { name: 'Режим доступа', exact: true })).toContainText('Полный доступ')
  })
  await audit.step('06-other-chat-alpha', async page => {
    await page.getByRole('button', { name: 'Новый чат', exact: true }).first().click()
    await expect(control(page)).toContainText(/alpha/)
    await send(page, 'UI_CHAT_B separate chat', 'audit-alpha')
    await page.getByRole('button', { name: 'Режим доступа', exact: true }).click()
    await page.getByRole('menuitemradio', { name: /С подтверждением/ }).click()
    await control(page).click()
    await page.getByRole('slider', { name: 'Уровень размышлений' }).focus()
    await page.getByRole('slider', { name: 'Уровень размышлений' }).press('Home')
    await page.keyboard.press('Escape')
  })
  await audit.step('07-theme-and-density', async page => {
    const dialog = await settings(page, 'Интерфейс')
    await dialog.getByRole('button', { name: 'Светлая', exact: true }).click()
    await dialog.getByRole('button', { name: 'Компактная', exact: true }).click()
    await expect(dialog.getByRole('button', { name: 'Светлая', exact: true })).toHaveClass(/selected/)
    await closeSettings(page)
  })
  await audit.step('08-add-provider', async page => {
    const dialog = await settings(page, 'Провайдеры')
    await dialog.getByRole('button', { name: 'Добавить провайдера', exact: true }).click()
    await dialog.getByLabel('ID провайдера', { exact: true }).fill('ui-added')
    await dialog.getByLabel('Название', { exact: true }).fill('UI Added')
    await dialog.getByLabel('Адрес API', { exact: true }).fill(`${audit.provider.url}/added`)
    await dialog.getByLabel('API ключ', { exact: true }).fill('fixture')
    await dialog.getByLabel('Получать каталог моделей от провайдера', { exact: true }).uncheck()
    await dialog.getByRole('button', { name: 'Добавить модель', exact: true }).click()
    await dialog.getByLabel('ID модели', { exact: true }).fill('ui-model')
    await dialog.getByLabel('Название модели', { exact: true }).fill('UI Added Model')
    await audit.capture('08-provider-form')
    await dialog.getByRole('button', { name: 'Сохранить изменения', exact: true }).click()
    await expect(dialog.getByText('UI Added', { exact: true }).first()).toBeVisible()
    await closeSettings(page)
  })
  await audit.step('09-add-mcp', async page => {
    const dialog = await settings(page, 'MCP')
    await dialog.getByRole('button', { name: 'Добавить сервер', exact: true }).click()
    await dialog.getByLabel('Название', { exact: true }).fill('ui-mcp')
    await dialog.getByRole('combobox', { name: 'Подключение', exact: true }).click()
    await page.getByRole('option', { name: 'HTTP', exact: true }).click()
    await dialog.getByLabel('Адрес сервера', { exact: true }).fill(`${audit.provider.url}/mcp`)
    await dialog.getByLabel('OAuth', { exact: true }).check()
    await dialog.getByLabel('OAuth', { exact: true }).uncheck()
    await audit.capture('09-mcp-form')
    await dialog.getByRole('button', { name: 'Сохранить', exact: true }).click()
    await expect(dialog.getByText('ui-mcp', { exact: true }).first()).toBeVisible()
    await closeSettings(page)
  })
  await audit.step('09b-add-lsp', async page => {
    const dialog = await settings(page, 'LSP')
    await dialog.getByRole('button', { name: 'Добавить сервер', exact: true }).click()
    await dialog.getByLabel('Название', { exact: true }).fill('ui-lsp')
    await dialog.getByLabel('Команда запуска', { exact: true }).fill('node')
    await dialog.getByLabel('Расширения файлов — через запятую', { exact: true }).fill('fixture')
    await dialog.getByLabel('Включено', { exact: true }).uncheck()
    await dialog.getByRole('button', { name: 'Сохранить', exact: true }).click()
    await expect(dialog.getByText('ui-lsp', { exact: true }).first()).toBeVisible()
    await closeSettings(page)
  })
  await audit.step('10-settings-tabs', async page => {
    const dialog = await settings(page, 'Горячие клавиши')
    for (const tab of ['Голос и речь', 'Изображения', 'LSP', 'Skills', 'Настройки Prime', 'О приложении']) {
      await dialog.getByRole('button', { name: tab, exact: true }).click()
      await expect(dialog).toBeVisible()
    }
    await closeSettings(page)
    await page.getByRole('textbox', { name: 'Сообщение Prime', exact: true }).fill('UI_DRAFT_PERSISTS')
    await page.getByRole('button', { name: 'Панель проектов', exact: true }).first().click()
    await page.locator('.session-row').filter({ hasText: 'UI_CHAT_A' }).first().locator('button').first().click()
    await expect(control(page)).toContainText(/audit[- ]beta/i)
    await page.getByRole('textbox', { name: 'Сообщение Prime', exact: true }).fill('UI_DRAFT_A')
  })
  await audit.close()
  await audit.launch()
  await audit.step('11-restart-current-chat', async page => {
    await expect(page.getByText('UI_CHAT_A initial', { exact: true })).toBeVisible()
    await expect(control(page)).toContainText(/audit[- ]beta/i)
    await expect(page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })).toHaveValue('UI_DRAFT_A')
    const prefs = JSON.parse(await readFile(join(audit.root, 'preferences.json'), 'utf8')).preferences
    expect(prefs.theme).toBe('light')
    expect(prefs.favoriteModels).toContainEqual({ provider: 'audit', model: 'audit-beta' })
    await page.getByRole('button', { name: 'Панель проектов', exact: true }).first().click()
    await page.locator('.session-row').filter({ hasText: 'UI_CHAT_B' }).first().locator('button').first().click()
    await expect(page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })).toHaveValue('UI_DRAFT_PERSISTS')
    await expect(page.getByRole('button', { name: 'Режим доступа', exact: true })).toContainText('С подтверждением')
    await send(page, 'UI_CHAT_B after full restart', 'audit-alpha')
  })
  await audit.step('12-first-chat-keeps-beta', async page => {
    const chat = page.locator('.session-row').filter({ hasText: 'UI_CHAT_A' }).first()
    await chat.locator('button').first().click()
    await expect(control(page)).toContainText(/audit[- ]beta/i)
    await expect(page.getByRole('button', { name: 'Режим доступа', exact: true })).toContainText('Полный доступ')
    await expect(control(page)).toContainText(/Максим|Высок/)
    await send(page, 'UI_CHAT_A after full restart', 'audit-beta')
  })
  await audit.step('13-provider-mcp-after-restart', async page => {
    const dialog = await settings(page, 'Провайдеры')
    await expect(dialog.getByText('UI Added', { exact: true }).first()).toBeVisible()
    await dialog.getByRole('button', { name: 'MCP', exact: true }).click()
    await expect(dialog.getByText('ui-mcp', { exact: true }).first()).toBeVisible()
    await dialog.getByRole('button', { name: 'LSP', exact: true }).click()
    await expect(dialog.getByText('ui-lsp', { exact: true }).first()).toBeVisible()
    await closeSettings(page)
  })
  await audit.step('14-project-tools', async page => {
    await page.getByRole('button', { name: 'Инструменты проекта', exact: true }).click()
    const dock = page.locator('.workbench-dock')
    await expect(dock).toBeVisible()
    const initial = (await dock.boundingBox()).height
    const tabs = dock.getByRole('tablist', { name: 'Инструменты проекта', exact: true })
    for (const tab of ['Терминал', 'Запуск', 'Автоматизации', 'Команды', 'GitHub', 'Окна']) {
      await tabs.getByRole('tab', { name: tab, exact: true }).click()
      await expect(tabs.getByRole('tab', { name: tab, exact: true })).toHaveAttribute('aria-selected', 'true')
      expect((await dock.boundingBox()).height).toBeLessThanOrEqual(initial + 5)
      await audit.capture(`14-tools-${tab}`)
    }
    await dock.getByRole('button', { name: 'Скрыть инструменты', exact: true }).click()
  })
  await audit.step('15-browser-tabs', async page => {
    await page.getByRole('button', { name: 'Браузер', exact: true }).click()
    const address = page.getByRole('textbox', { name: 'Адрес страницы', exact: true })
    await address.fill(`${audit.provider.url}/fixture`)
    await address.press('Enter')
    await expect(page.getByRole('tablist', { name: 'Вкладки браузера', exact: true })).toContainText('Native browser fixture')
    await page.getByRole('button', { name: 'Новая вкладка', exact: true }).click()
    await expect(page.getByRole('tablist', { name: 'Вкладки браузера', exact: true }).getByRole('tab')).toHaveCount(2)
    await page.getByRole('tablist', { name: 'Вкладки браузера', exact: true }).getByRole('tab').first().click()
    await expect(address).toHaveValue(`${audit.provider.url}/fixture`)
  })
  await audit.step('16-added-provider-and-rename', async page => {
    await page.getByRole('button', { name: 'Файлы', exact: true }).click()
    await openModels(page)
    await page.getByRole('option', { name: /UI Added Model/ }).first().click()
    await send(page, 'UI_CHAT_A added provider', 'ui-model')
    expect(audit.provider.requests.some(request => request.stream && request.model === 'ui-model' && request.fixturePath.startsWith('/added/'))).toBe(true)
    const dialog = await settings(page, 'Провайдеры')
    await dialog.locator('.provider-card').filter({ hasText: 'UI Added' }).getByRole('button', { name: 'Изменить', exact: true }).click()
    await dialog.getByLabel('ID провайдера', { exact: true }).fill('ui-renamed')
    await dialog.getByLabel('Название', { exact: true }).fill('UI Renamed')
    await dialog.getByRole('button', { name: 'Сохранить изменения', exact: true }).click()
    await expect(dialog.getByText('UI Renamed', { exact: true }).first()).toBeVisible()
    await closeSettings(page)
    await send(page, 'UI_CHAT_A renamed provider', 'ui-model')
  })
  await auditProject(audit)
} finally {
  await audit.dispose()
}


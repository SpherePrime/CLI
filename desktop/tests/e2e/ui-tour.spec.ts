import { test, type Page } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

async function shot(page: Page, prefix: string, name: string) {
  await page.waitForTimeout(400)
  await page.screenshot({ path: `test-results/tour/${prefix}-${name}.png` })
}

async function attempt(action: () => Promise<unknown>) {
  try {
    await action()
  } catch (error) {
    console.log('skip', String(error).split('\n')[0])
  }
}

const sizes = [
  ['wide', 1280, 820],
  ['narrow', 700, 760]
] as const

for (const [prefix, width, height] of sizes)
  test(`captures every surface of the desktop interface (${prefix})`, async () => {
    test.setTimeout(300000)
    const { app, page, provider } = await launchWorkbench()
    page.setDefaultTimeout(8000)
    try {
      await app.evaluate(
        ({ BrowserWindow }, size) => {
          const window = BrowserWindow.getAllWindows()[0]
          window.setMinimumSize(420, 420)
          window.setSize(size[0], size[1])
        },
        [width, height]
      )
      await shot(page, prefix, '01-start')
      await attempt(async () => {
        await page.getByRole('button', { name: 'Новый чат', exact: true }).first().click()
        await shot(page, prefix, '02-welcome')
      })
      await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('Проверка интерфейса')
      await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
      await page.locator('.message.assistant').first().waitFor({ timeout: 30000 })
      await shot(page, prefix, '03-chat')
      await attempt(async () => {
        await page
          .getByRole('textbox', { name: 'Сообщение Prime' })
          .fill('TOOL_FIXTURE Создай файл')
        await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
        await page.getByRole('heading', { name: 'Разрешить действие?' }).waitFor({ timeout: 15000 })
        await shot(page, prefix, '04-permission')
        await page.getByRole('button', { name: 'Разрешить', exact: true }).click()
        await page.waitForTimeout(1500)
        await shot(page, prefix, '05-tool')
      })
      for (const [index, name] of ['Файлы', 'Изменения', 'Браузер'].entries())
        await attempt(async () => {
          await page.locator(`.inspector-tabs button[aria-label="${name}"]`).click()
          await shot(page, prefix, `06-inspector-${index}`)
        })
      await attempt(async () => {
        await page.keyboard.press('Control+`')
        await page.locator('.workbench-dock').waitFor()
        await page.waitForTimeout(1500)
        await shot(page, prefix, '07-dock-terminal')
        for (const [index, name] of [
          'Запуск',
          'Автоматизации',
          'Команды',
          'GitHub',
          'Окна'
        ].entries())
          await attempt(async () => {
            await page.getByRole('tab', { name, exact: true }).click()
            await shot(page, prefix, `08-dock-${index}`)
          })
        await page.getByRole('button', { name: 'Скрыть инструменты', exact: true }).click()
      })
      await attempt(async () => {
        await page
          .getByRole('button', { name: 'Модель и уровень размышлений', exact: true })
          .click()
        await shot(page, prefix, '09-model')
        await page.keyboard.press('Escape')
      })
      await attempt(async () => {
        await page.keyboard.press('Control+k')
        await shot(page, prefix, '10-commands')
        await page.keyboard.press('Escape')
      })
      await attempt(async () => {
        await page.keyboard.press('Control+Comma')
        await page.locator('dialog[open]').waitFor()
        await shot(page, prefix, '11-settings-0')
        for (const [index, name] of [
          'Горячие клавиши',
          'Провайдеры',
          'Голос',
          'MCP',
          'LSP',
          'Skills',
          'Настройки Prime',
          'О приложении'
        ].entries())
          await attempt(async () => {
            await page
              .locator('dialog[open]')
              .getByRole('button', { name, exact: true })
              .first()
              .click()
            await shot(page, prefix, `11-settings-${index + 1}`)
          })
        await page.keyboard.press('Escape')
        await page.locator('dialog[open]').waitFor({ state: 'detached' })
      })
      for (const [name, query] of [
        ['12-jobs', 'Фоновые задачи'],
        ['13-plugins', 'Плагины'],
        ['14-remote', 'SSH']
      ])
        await attempt(async () => {
          await page.keyboard.press('Control+k')
          await page.keyboard.type(query)
          await page.keyboard.press('Enter')
          await page.locator('dialog[open]').waitFor()
          await shot(page, prefix, name)
          await page.keyboard.press('Escape')
          await page.locator('dialog[open]').waitFor({ state: 'detached' })
        })
    } finally {
      await app.close()
      await provider.close()
    }
  })

import { test } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

const out = (name: string) => `test-results/design/${name}.png`

test('captures key surfaces for design review', async () => {
  test.setTimeout(240000)
  const fixture = await launchWorkbench()
  const { page, project } = fixture
  const dock = async (tab: string) => {
    await page.getByRole('tab', { name: tab, exact: true }).click()
    await page.waitForTimeout(400)
  }
  try {
    for (const theme of ['dark', 'light'] as const) {
      await page.evaluate((theme) => window.prime.preferences({ theme }), theme)
      await page.reload()
      await page.getByRole('textbox', { name: 'Сообщение Prime', exact: true }).waitFor()
      await page.setViewportSize({ width: 1440, height: 900 })
      await page.screenshot({ path: out(`${theme}-home`) })
      await page.getByRole('textbox', { name: 'Сообщение Prime', exact: true }).fill('Расскажи о проекте')
      await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
      await page.getByText('через встроенный CLI.').first().waitFor({ timeout: 20000 })
      await page.screenshot({ path: out(`${theme}-chat`) })
      await page.getByRole('button', { name: 'Терминал проекта', exact: true }).click()
      await page.getByRole('button', { name: 'Развернуть панель', exact: true }).click().catch(() => {})
      for (const [tab, name] of [
        ['Терминал', 'terminal'],
        ['Запуск', 'launch'],
        ['Автоматизации', 'automations'],
        ['Команды', 'teams'],
        ['GitHub', 'github'],
        ['Окна', 'windows']
      ])
        await dock(tab).then(() => page.screenshot({ path: out(`${theme}-dock-${name}`) }))
      await page.keyboard.press('Escape')
      await page.getByRole('button', { name: 'Скрыть инструменты' }).click().catch(() => {})
      await page.getByRole('button', { name: /Фоновые задачи/ }).first().click()
      await page.waitForTimeout(500)
      await page.screenshot({ path: out(`${theme}-jobs`) })
      await page.keyboard.press('Escape')
      await page.getByRole('button', { name: 'SSH-серверы' }).first().click()
      await page.waitForTimeout(300)
      await page.screenshot({ path: out(`${theme}-remote`) })
      await page.keyboard.press('Escape')
      await page.getByRole('button', { name: 'Настройки' }).first().click()
      await page.getByRole('button', { name: 'Голос', exact: true }).click()
      await page.waitForTimeout(300)
      await page.screenshot({ path: out(`${theme}-settings-voice`) })
      await page.keyboard.press('Escape')
      await page.setViewportSize({ width: 860, height: 620 })
      await page.waitForTimeout(300)
      await page.screenshot({ path: out(`${theme}-small`) })
      await page.getByRole('button', { name: 'Терминал проекта', exact: true }).click()
      await dock('Запуск')
      await page.screenshot({ path: out(`${theme}-small-launch`) })
      await page.getByRole('button', { name: 'Скрыть инструменты' }).click().catch(() => {})
    }
    void project
  } finally {
    await fixture.close()
  }
})

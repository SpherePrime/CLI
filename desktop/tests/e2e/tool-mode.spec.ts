import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('tool discovery mode persists and changes the tools sent to the provider', async () => {
  const fixture = await launchWorkbench()
  let page = fixture.page
  const settings = async () => {
    await page.getByRole('button', { name: 'Настройки', exact: true }).click()
    await page
      .locator('.settings-modal nav')
      .getByRole('button', { name: 'Настройки Prime', exact: true })
      .click()
  }
  try {
    await settings()
    const mode = () => page.getByRole('combobox', { name: 'Режим инструментов', exact: true })
    await expect(mode()).toHaveText('Обычный')
    await mode().click()
    await page.getByRole('option', { name: 'SmartTools', exact: true }).click()
    await expect(mode()).toHaveText('SmartTools')
    await expect
      .poll(() =>
        page.evaluate(
          async (id) => (await window.prime.call(id, 'config')).options.smart_tools,
          fixture.project.id
        )
      )
      .toBe(true)
    ;({ page } = await fixture.restart())
    await settings()
    await expect(mode()).toHaveText('SmartTools')
    await page
      .locator('.settings-modal')
      .getByRole('button', { name: 'Закрыть', exact: true })
      .click()
    await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('SMART_MODE_CHECK')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(page.locator('.message.assistant').first()).toBeVisible()
    await expect(page.locator('.composer .send.stop')).toHaveCount(0)
    const hasSearch = (prompt: any) =>
      prompt.tools?.some((tool: any) => tool.function.name === 'search_skills')
    expect(fixture.provider.prompts.some(hasSearch)).toBe(true)
    await settings()
    await mode().click()
    await page.getByRole('option', { name: 'Обычный', exact: true }).click()
    await expect(mode()).toHaveText('Обычный')
    await page
      .locator('.settings-modal')
      .getByRole('button', { name: 'Закрыть', exact: true })
      .click()
    const start = fixture.provider.prompts.length
    await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('STANDARD_MODE_CHECK')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect.poll(() => fixture.provider.prompts.length).toBeGreaterThan(start)
    expect(fixture.provider.prompts.slice(start).some(hasSearch)).toBe(false)
  } finally {
    await fixture.close()
  }
})

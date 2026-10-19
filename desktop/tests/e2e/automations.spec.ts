import { expect, test } from '@playwright/test'
import type { AutomationResponse } from '../../src/shared/automations'
import { launchWorkbench } from '../fixtures/workbench-app'

test('automations run real sessions, stop work and survive app restart', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  let { page } = fixture
  const open = async () => {
    await page.keyboard.press('Control+k')
    await page
      .getByRole('combobox', { name: 'Найти действие или чат', exact: true })
      .fill('Автоматизации')
    await page.getByRole('option', { name: 'Автоматизации', exact: true }).click()
    await expect(page.locator('.automation-panel')).toBeVisible()
  }
  const snapshot = () =>
    page.evaluate(async (root) => {
      const project = (await window.prime.projects()).find(
        (item) => item.path.toLowerCase() === root.toLowerCase()
      )!
      return window.prime.automation(project.id, { action: 'list', root: 'C:/foreign-root' })
    }, fixture.project.path)
  try {
    await open()
    await page.getByRole('button', { name: 'Новая автоматизация', exact: true }).click()
    await page.getByRole('textbox', { name: 'Название', exact: true }).fill('Обзор проекта')
    await page
      .getByRole('textbox', { name: 'Задача для Prime', exact: true })
      .fill('AUTOMATION_FIXTURE Проверь состояние проекта')
    await page.getByRole('spinbutton', { name: 'Интервал, минуты', exact: true }).fill('120')
    await page.getByRole('button', { name: 'Сохранить', exact: true }).click()
    const rule = page.locator('.automation-rule').filter({ hasText: 'Обзор проекта' })
    await expect(rule).toContainText('Следующий запуск:')
    await rule.getByRole('button', { name: 'Запустить', exact: true }).click()
    await expect
      .poll(async () => (await snapshot()).runs[0]?.status, { timeout: 30000 })
      .toBe('completed')
    const completed = (await snapshot()).runs[0]
    const realRun = await page.evaluate(
      async ({ id, runId }) => window.prime.call(id, 'runs', { runId }),
      { id: fixture.project.id, runId: completed.id }
    )
    expect(realRun.session_id).toBe(completed.sessionId)
    expect(
      fixture.provider.prompts.some((body) => JSON.stringify(body).includes('AUTOMATION_FIXTURE'))
    ).toBe(true)
    await page
      .locator('.automation-history')
      .getByRole('button', { name: 'Открыть чат', exact: true })
      .click()
    await expect(page.locator('.workbench-dock')).toHaveCount(0)
    const selected = await page.evaluate(
      async (id) =>
        (await window.prime.projects()).find((item) => item.id === id)?.currentSessionId,
      fixture.project.id
    )
    expect(selected).toBe(completed.sessionId)
    await open()
    await rule.getByRole('button', { name: 'Приостановить', exact: true }).click()
    await expect(rule).toContainText('Расписание приостановлено')
    await page.getByRole('button', { name: 'Новая автоматизация', exact: true }).click()
    await page.getByRole('textbox', { name: 'Название', exact: true }).fill('Остановка задачи')
    await page.getByRole('textbox', { name: 'Задача для Prime', exact: true }).fill('HOLD_FIXTURE')
    await page.getByRole('checkbox', { name: 'Включено', exact: true }).uncheck()
    await page.getByRole('button', { name: 'Сохранить', exact: true }).click()
    const manual = await page.evaluate(async (id) => {
      const session = await window.prime.call(id, 'createSession', { title: 'Parallel manual run' })
      const runId = crypto.randomUUID()
      await window.prime.call(id, 'send', {
        sessionId: session.id,
        runId,
        prompt: 'HOLD_FIXTURE manual'
      })
      return { sessionId: session.id, runId }
    }, fixture.project.id)
    await page
      .locator('.automation-rule')
      .filter({ hasText: 'Остановка задачи' })
      .getByRole('button', { name: 'Запустить', exact: true })
      .click()
    await expect(page.getByRole('button', { name: 'Остановить запуск', exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Остановить запуск', exact: true }).click()
    await expect.poll(async () => (await snapshot()).runs[0]?.status).toBe('cancelled')
    await page.evaluate(
      async ({ id, sessionId }) => window.prime.call(id, 'cancel', { sessionId }),
      { id: fixture.project.id, sessionId: manual.sessionId }
    )
    await expect
      .poll(async () =>
        page.evaluate(
          async ({ id, runId }) => (await window.prime.call(id, 'runs', { runId })).status,
          { id: fixture.project.id, runId: manual.runId }
        )
      )
      .toBe('cancelled')
    ;({ page } = await fixture.restart())
    await open()
    const restored: AutomationResponse = await snapshot()
    expect(restored.rules).toHaveLength(2)
    expect(restored.rules.every((item) => !item.enabled)).toBe(true)
    expect(restored.runs.map((item) => item.status)).toEqual(['cancelled', 'completed'])
    await expect(page.locator('.automation-rule')).toHaveCount(2)
    await page.setViewportSize({ width: 1000, height: 600 })
    await expect
      .poll(() =>
        page
          .locator('.automation-panel')
          .evaluate((element) => element.scrollWidth - element.clientWidth)
      )
      .toBeLessThanOrEqual(1)
    await page.screenshot({ path: 'test-results/workbench-automations.png' })
    expect(fixture.errors).toEqual([])
  } finally {
    await fixture.close()
  }
})

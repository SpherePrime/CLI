import { test, expect } from '@playwright/test'
import type { TeamResponse } from '../../src/shared/teams'
import { launchWorkbench } from '../fixtures/workbench-app'

test('agent team runs members in parallel and stops one without touching others', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  const { page, project } = fixture
  try {
    await page.getByRole('button', { name: 'Быстрые действия', exact: true }).click()
    await page.getByRole('option', { name: 'Команды агентов' }).click()
    await page.getByRole('button', { name: 'Новая группа' }).click()
    await page.getByLabel('Название группы').fill('Fixture team')
    await page.getByLabel('Задача исполнителя 1').fill('Hold')
    await page.getByLabel('Инструкция исполнителя 1').fill('HOLD_FIXTURE wait')
    await page.getByLabel('Задача исполнителя 2').fill('Answer')
    await page.getByLabel('Инструкция исполнителя 2').fill('Reply shortly')
    await page.getByRole('button', { name: 'Запустить группу' }).click()
    const list = () =>
      page.evaluate((id) => window.prime.team(id, { action: 'list' }), project.id) as Promise<TeamResponse>
    const status = async (title: string) =>
      (await list()).teams[0]?.tasks.find((task) => task.title === title)?.status
    await expect.poll(() => status('Hold'), { timeout: 30000 }).toBe('running')
    await expect.poll(() => status('Answer'), { timeout: 30000 }).toBe('completed')
    await page.getByRole('button', { name: 'Остановить исполнителя Hold' }).click()
    await expect.poll(() => status('Hold'), { timeout: 30000 }).toBe('cancelled')
    expect(await status('Answer')).toBe('completed')
    await page.screenshot({ path: 'test-results/teams.png' })
    await page.getByRole('button', { name: 'Открыть чат исполнителя Answer' }).click()
    await expect(page.getByText('через встроенный CLI.').first()).toBeVisible({ timeout: 15000 })
  } finally {
    await fixture.close()
  }
})

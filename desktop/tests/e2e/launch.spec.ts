import { test, expect } from '@playwright/test'
import { resolve } from 'node:path'
import type { LaunchResponse } from '../../src/shared/launch'
import { launchWorkbench } from '../fixtures/workbench-app'

test('launches a real app, opens preview, collects diagnostics and stops it', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  const { page, project } = fixture
  try {
    await page.getByRole('button', { name: 'Терминал проекта', exact: true }).click()
    await page.getByRole('tab', { name: 'Запуск' }).click()
    await page.getByRole('button', { name: 'Новая команда' }).click()
    await page.getByLabel('Название').fill('Fixture app')
    await page.getByLabel('Команда').fill(`node "${resolve('tests/fixtures/launch-app.cjs')}"`)
    await page.getByRole('button', { name: 'Сохранить' }).click()
    await page.getByRole('button', { name: 'Запустить', exact: true }).click()
    const list = () =>
      page.evaluate((id) => window.prime.launch(id, { action: 'list' }), project.id) as Promise<LaunchResponse>
    await expect.poll(async () => (await list()).processes[0]?.url ?? '', { timeout: 30000 }).toMatch(/^http:\/\/localhost:\d+\/$/)
    await expect.poll(async () => (await page.evaluate(() => window.prime.browserState())).title !== undefined && (await page.evaluate(() => window.prime.browserState())).url, { timeout: 20000 }).toMatch(/localhost/)
    await expect(page.getByText('FIXTURE_BROWSER_ERROR').first()).toBeVisible({ timeout: 20000 })
    await page.screenshot({ path: 'test-results/launch.png' })
    await page.getByRole('button', { name: 'Передать агенту' }).click()
    await expect(page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })).toHaveValue(/FIXTURE_BROWSER_ERROR[\s\S]*|Local: http/)
    const terminalId = (await list()).processes[0].terminalId
    await page.getByRole('tab', { name: 'Запуск' }).click()
    await page.getByRole('button', { name: 'Остановить', exact: true }).click()
    await expect.poll(async () => (await list()).processes.length).toBe(0)
    const terminals = await page.evaluate((id) => window.prime.terminal(id, { action: 'list' }), project.id)
    expect((terminals as { id: string }[]).some((item) => item.id === terminalId)).toBe(false)
  } finally {
    await fixture.close()
  }
})

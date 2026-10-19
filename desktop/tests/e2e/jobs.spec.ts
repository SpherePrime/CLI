import { test, expect } from '@playwright/test'
import type { Job } from '../../src/shared/jobs'
import { launchWorkbench } from '../fixtures/workbench-app'

test('background center shows work from several projects, stops one and opens its chat', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  const { page, project } = fixture
  try {
    const chats = await page.evaluate(() => window.prime.openChat())
    const start = (id: string, title: string, prompt: string) =>
      page.evaluate(
        async ({ id, title, prompt }) => {
          const session = await window.prime.call(id, 'createSession', { title })
          await window.prime.call(id, 'send', { sessionId: session.id, runId: crypto.randomUUID(), prompt })
          return session.id as string
        },
        { id, title, prompt }
      )
    const longSession = await start(project.id, 'Long task', 'HOLD_FIXTURE wait')
    const quickSession = await start(chats.id, 'Quick task', 'Reply shortly')
    await page.evaluate((id) => window.prime.terminal(id, { action: 'create' }), project.id)
    const jobs = () => page.evaluate(() => window.prime.jobs({ action: 'list' })) as Promise<Job[]>
    await expect.poll(async () => (await jobs()).find((job) => job.sessionId === longSession)?.status, { timeout: 30000 }).toBe('running')
    await expect.poll(async () => (await jobs()).find((job) => job.sessionId === quickSession)?.status, { timeout: 30000 }).toBe('completed')
    expect((await jobs()).some((job) => job.kind === 'terminal' && job.status === 'running')).toBe(true)
    await page.getByRole('button', { name: /Фоновые задачи/ }).first().click()
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByText('Long task')).toBeVisible()
    await dialog.getByRole('tab', { name: /Все/ }).last().click()
    const quickTitle = (await jobs()).find((job) => job.sessionId === quickSession)!.title
    await expect(dialog.getByText(quickTitle).first()).toBeVisible()
    await page.screenshot({ path: 'test-results/jobs.png' })
    await dialog.getByRole('button', { name: 'Остановить Long task' }).click()
    await expect.poll(async () => (await jobs()).find((job) => job.sessionId === longSession)?.status, { timeout: 20000 }).not.toBe('running')
    await dialog.getByRole('button', { name: `Открыть ${quickTitle}` }).first().click()
    await expect(page.getByText('через встроенный CLI.').first()).toBeVisible({ timeout: 15000 })
  } finally {
    await fixture.close()
  }
})

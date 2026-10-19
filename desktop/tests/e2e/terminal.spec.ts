import { test, expect } from '@playwright/test'
import type { TerminalRead, TerminalSnapshot } from '../../src/shared/terminal'
import { launchWorkbench } from '../fixtures/workbench-app'

test('project terminal round trip, agent handover and remount use real ConPTY', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  const { page, app, project, errors } = fixture
  try {
    await page.getByRole('button', { name: 'Терминал проекта', exact: true }).click()
    await page.setViewportSize({ width: 1000, height: 600 })
    await expect
      .poll(async () =>
        page
          .locator('.workbench-dock')
          .evaluate((element) => element.getBoundingClientRect().bottom)
      )
      .toBeLessThanOrEqual(600)
    const composer = page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })
    await composer.fill('Draft line\n'.repeat(20))
    await expect
      .poll(async () =>
        page
          .locator('.workbench-dock')
          .evaluate((element) => element.getBoundingClientRect().bottom)
      )
      .toBeLessThanOrEqual(600)
    await composer.fill('')
    await page.getByRole('button', { name: 'Развернуть панель', exact: true }).click()
    await page.getByRole('button', { name: 'Новый терминал', exact: true }).first().click()
    await expect(page.locator('.terminal-emulator .xterm')).toBeVisible()
    const list = () =>
      page.evaluate(
        async (id) => (await window.prime.terminal(id, { action: 'list' })) as TerminalSnapshot[],
        project.id
      )
    await expect.poll(async () => (await list()).length).toBe(1)
    const terminal = (await list())[0]
    await page.locator('.terminal-emulator .xterm-helper-textarea').press('Enter')
    await page.keyboard.insertText("Write-Output ('DESKTOP_' + 'TERMINAL_OK')")
    await page.keyboard.press('Enter')
    const read = () =>
      page.evaluate(
        async ({ workspaceId, id }) =>
          (await window.prime.terminal(workspaceId, { action: 'read', id })) as TerminalRead,
        { workspaceId: project.id, id: terminal.id }
      )
    await expect.poll(async () => (await read()).output).toContain('DESKTOP_TERMINAL_OK')
    await page.getByRole('button', { name: 'Передать агенту', exact: true }).click()
    await expect.poll(async () => (await read()).terminal.owner).toBe('agent')
    await expect(page.locator('.terminal-agent-notice')).toBeVisible()
    const endpoint = await app.evaluate(() => ({
      url: process.env.PRIME_DESKTOP_BROWSER_URL!,
      token: process.env.PRIME_DESKTOP_BROWSER_TOKEN!
    }))
    const request = async (name: string, args: Record<string, unknown>) => {
      const response = await fetch(endpoint.url, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${endpoint.token}`,
          'X-Prime-Workspace': Buffer.from(project.path).toString('base64url')
        },
        body: JSON.stringify({
          jsonrpc: '2.0',
          id: 1,
          method: 'tools/call',
          params: { name, arguments: args }
        })
      })
      return response.json()
    }
    expect(
      (
        await request('terminal_write', {
          id: terminal.id,
          data: "Write-Output ('AGENT_' + 'TERMINAL_OK')\r"
        })
      ).result.isError
    ).not.toBe(true)
    await expect.poll(async () => (await read()).output).toContain('AGENT_TERMINAL_OK')
    await page.getByRole('button', { name: 'Вернуть управление', exact: true }).click()
    expect((await request('terminal_write', { id: terminal.id, data: '\r' })).result.isError).toBe(
      true
    )
    await page.getByRole('button', { name: 'Скрыть инструменты', exact: true }).click()
    await page.getByRole('button', { name: 'Терминал проекта', exact: true }).click()
    await expect(page.locator('.terminal-emulator .xterm')).toBeVisible()
    expect((await list())[0].id).toBe(terminal.id)
    await page.locator('.workbench-dock').evaluate((element) =>
      Promise.all(
        element
          .getAnimations({ subtree: true })
          .filter((animation) => animation.effect?.getTiming().iterations !== Infinity)
          .map((animation) => animation.finished)
      )
    )
    await page.screenshot({ path: 'test-results/workbench-terminal.png' })
    await page.getByRole('button', { name: /Закрыть терминал powershell/ }).click()
    await expect.poll(async () => (await list()).length).toBe(0)
    expect(errors).toEqual([])
  } finally {
    await fixture.close()
  }
})

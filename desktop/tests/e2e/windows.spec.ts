import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('lists a test window, captures it and keeps agent access view-only and revocable', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  const { page, app, project } = fixture
  try {
    await app.evaluate(async ({ BrowserWindow }) => {
      const fixtureWindow = new BrowserWindow({ width: 420, height: 300, title: 'Window Fixture', show: true })
      await fixtureWindow.loadURL('data:text/html,<title>Window Fixture</title><body style="background:%2333aa66"><h1>Fixture</h1></body>')
      fixtureWindow.setTitle('Window Fixture')
    })
    const tool = (name: string) =>
      app.evaluate(
        async (_electron, { name, root }) => {
          const response = await fetch(process.env.PRIME_DESKTOP_BROWSER_URL!, {
            method: 'POST',
            headers: {
              Authorization: `Bearer ${process.env.PRIME_DESKTOP_BROWSER_TOKEN}`,
              'Content-Type': 'application/json',
              'x-prime-workspace': Buffer.from(root).toString('base64url')
            },
            body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/call', params: { name, arguments: {} } })
          })
          return response.json()
        },
        { name, root: fixture.root }
      )
    expect((await tool('window_view')).result.isError).toBe(true)
    await page.getByRole('button', { name: 'Терминал проекта', exact: true }).click()
    await page.getByRole('tab', { name: 'Окна' }).click()
    await page.getByRole('button', { name: /Window Fixture/ }).click()
    await expect(page.locator('.windows-preview img')).toBeVisible({ timeout: 15000 })
    const session = await page.evaluate((id) => window.prime.call(id, 'createSession', { title: 'Window chat' }), project.id)
    const state = await page.evaluate(
      async ({ id, sessionId }) => {
        const windows = await window.prime.windows(id, { action: 'list' })
        const target = windows.find((item: { title: string }) => item.title === 'Window Fixture')
        return window.prime.windows(id, { action: 'share', hwnd: target.hwnd, sessionId })
      },
      { id: project.id, sessionId: session.id }
    )
    expect(state.share.title).toBe('Window Fixture')
    const denied = await tool('window_view')
    expect(denied.result.isError).toBe(true)
    expect(denied.result.content[0].text).toContain('different chat')
    await page.screenshot({ path: 'test-results/windows.png' })
    await page.getByRole('button', { name: 'Отозвать' }).click()
    await expect(page.locator('.windows-share')).toHaveCount(0)
    expect((await tool('window_view')).result.content[0].text).toContain('not shared')
  } finally {
    await fixture.close()
  }
})

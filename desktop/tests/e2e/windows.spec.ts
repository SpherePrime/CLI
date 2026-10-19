import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('requests scoped window access, captures and controls the approved fixture, and revokes immediately', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  const { page, app, project } = fixture
  try {
    const target = await app.evaluate(async ({ BrowserWindow }) => {
      const fixtureWindow = new BrowserWindow({
        width: 500,
        height: 360,
        title: 'Window Fixture',
        show: true
      })
      await fixtureWindow.loadURL(
        'data:text/html,' +
          encodeURIComponent(
            '<title>Window Fixture</title><body style="background:#183b30;color:white"><h1>Window fixture</h1><input id="fixture-input" aria-label="Fixture input"></body>'
          )
      )
      fixtureWindow.setTitle('Window Fixture')
      return {
        hwnd: fixtureWindow.getNativeWindowHandle().readBigUInt64LE().toString(),
        id: fixtureWindow.id
      }
    })
    const tool = (name: string, args: Record<string, unknown> = {}) =>
      app.evaluate(
        async (_electron, { name, args, root }) => {
          const response = await fetch(process.env.PRIME_DESKTOP_BROWSER_URL!, {
            method: 'POST',
            headers: {
              Authorization: `Bearer ${process.env.PRIME_DESKTOP_BROWSER_TOKEN}`,
              'Content-Type': 'application/json',
              'x-prime-workspace': Buffer.from(root).toString('base64url')
            },
            body: JSON.stringify({
              jsonrpc: '2.0',
              id: 1,
              method: 'tools/call',
              params: { name, arguments: args }
            })
          })
          return response.json()
        },
        { name, args, root: fixture.root }
      )
    expect((await tool('window_view')).result.isError).toBe(true)
    const listed = JSON.parse((await tool('window_list')).result.content[0].text)
    expect(listed.some((item: { hwnd: string }) => item.hwnd === target.hwnd)).toBe(true)
    await page
      .getByRole('textbox', { name: 'Сообщение Prime', exact: true })
      .fill('HOLD_FIXTURE window access acceptance')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect
      .poll(async () =>
        (await page.evaluate((id) => window.prime.call(id, 'sessions'), project.id)).some(
          (item: { is_busy: boolean }) => item.is_busy
        )
      )
      .toBe(true)
    const requestView = await tool('window_request_access', { hwnd: target.hwnd, mode: 'view' })
    expect(requestView.result.isError).not.toBe(true)
    expect(JSON.parse(requestView.result.content[0].text).status).toBe('awaiting_user')
    await expect(page.getByRole('region', { name: 'Запрос просмотра окна' })).toBeVisible()
    await page
      .getByRole('region', { name: 'Запрос просмотра окна' })
      .getByRole('button', { name: 'Разрешить просмотр', exact: true })
      .click()
    await expect(page.locator('.windows-share')).toContainText('Просмотр разрешён')
    expect(
      (await tool('window_view')).result.content.some(
        (item: { type: string }) => item.type === 'image'
      )
    ).toBe(true)
    expect((await tool('window_action', { action: 'focus' })).result.isError).toBe(true)
    await tool('window_request_access', { hwnd: target.hwnd, mode: 'control' })
    await page
      .getByRole('region', { name: 'Запрос управления окном' })
      .getByRole('button', { name: 'Разрешить управление', exact: true })
      .click()
    await expect(page.locator('.windows-share')).toContainText('Управление разрешено')
    expect(
      await app.evaluate(({ globalShortcut }) => globalShortcut.isRegistered('Control+Alt+Escape'))
    ).toBe(true)
    expect((await tool('window_action', { action: 'focus' })).result.isError).not.toBe(true)
    const windowState = JSON.parse((await tool('window_view')).result.content[0].text)
    const point = await app.evaluate(
      async ({ BrowserWindow }, { id, bounds }) => {
        const fixtureWindow = BrowserWindow.fromId(id)!
        const rect = await fixtureWindow.webContents.executeJavaScript(
          '(()=>{const r=document.getElementById("fixture-input").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()'
        )
        const content = fixtureWindow.getContentBounds()
        return {
          x: Math.round(content.x + rect.x - bounds.x),
          y: Math.round(content.y + rect.y - bounds.y)
        }
      },
      { id: target.id, bounds: windowState.window.bounds }
    )
    expect((await tool('window_action', { action: 'click', ...point })).result.isError).not.toBe(
      true
    )
    await expect
      .poll(() =>
        app.evaluate(
          async ({ BrowserWindow }, id) =>
            BrowserWindow.fromId(id)!.webContents.executeJavaScript('document.activeElement.id'),
          target.id
        )
      )
      .toBe('fixture-input')
    expect(
      (await tool('window_action', { action: 'type', text: 'Окно под управлением' })).result.isError
    ).not.toBe(true)
    await expect
      .poll(() =>
        app.evaluate(
          async ({ BrowserWindow }, id) =>
            BrowserWindow.fromId(id)!.webContents.executeJavaScript(
              'document.getElementById("fixture-input").value'
            ),
          target.id
        )
      )
      .toBe('Окно под управлением')
    await page
      .getByRole('button', { name: /Window Fixture/ })
      .first()
      .click()
    await expect(page.locator('.windows-preview img')).toBeVisible()
    await page.screenshot({ path: 'test-results/windows.png' })
    await page.locator('.windows-revoke').click()
    await expect(page.locator('.windows-share')).toHaveCount(0)
    expect(
      (await tool('window_action', { action: 'type', text: 'forbidden' })).result.isError
    ).toBe(true)
    expect((await tool('window_view')).result.content[0].text).toContain('not shared')
    expect(fixture.errors).toEqual([])
  } finally {
    await fixture.close()
  }
})

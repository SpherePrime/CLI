import { test, expect } from '@playwright/test'
import { createServer } from 'node:http'
import { launchWorkbench } from '../fixtures/workbench-app'

test('picks a page element, cancels with Escape and sends it to the chat', async () => {
  test.setTimeout(120000)
  const server = createServer((_request, response) => {
    response.setHeader('Content-Type', 'text/html; charset=utf-8')
    response.end(
      '<html><body style="margin:0"><button id="buy" style="position:absolute;left:20px;top:20px;width:160px;height:48px;background:#2a6">Купить</button></body></html>'
    )
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const url = `http://127.0.0.1:${(server.address() as { port: number }).port}/`
  const fixture = await launchWorkbench()
  const { page, app } = fixture
  const view = (script: string) =>
    app.evaluate(({ webContents }, { url, script }) => {
      const contents = webContents.getAllWebContents().find((item) => item.getURL() === url)!
      return contents.executeJavaScript(script)
    }, { url, script })
  const click = (x: number, y: number) =>
    app.evaluate(({ webContents }, { url, x, y }) => {
      const contents = webContents.getAllWebContents().find((item) => item.getURL() === url)!
      contents.sendInputEvent({ type: 'mouseMove', x, y })
      contents.sendInputEvent({ type: 'mouseDown', x, y, button: 'left', clickCount: 1 })
      contents.sendInputEvent({ type: 'mouseUp', x, y, button: 'left', clickCount: 1 })
    }, { url, x, y })
  try {
    await page.evaluate((url) => window.prime.browserNavigate(url), url)
    const pickButton = page.getByRole('button', { name: 'Выбрать элемент' })
    await expect(pickButton).toBeEnabled()
    await pickButton.click()
    await expect(page.getByText('Нажмите на элемент. С Ctrl — несколько. Esc — отмена.')).toBeVisible()
    await expect.poll(() => view('Boolean(window.__primePicker)')).toBe(true)
    await page.keyboard.press('Escape')
    await expect(page.getByText('Нажмите на элемент. С Ctrl — несколько. Esc — отмена.')).toHaveCount(0)
    expect(await view('document.querySelectorAll("[data-prime-picker]").length')).toBe(0)

    await pickButton.click()
    await expect.poll(() => view('Boolean(window.__primePicker)')).toBe(true)
    await click(60, 40)
    const note = page.getByLabel('Замечание к элементу')
    await expect(note).toBeVisible({ timeout: 10000 })
    await expect(page.locator('.element-pick code')).toHaveText('#buy')
    await expect(page.locator('.element-pick img')).toBeVisible()
    expect(await view('document.querySelectorAll("[data-prime-picker]").length')).toBe(0)
    await page.screenshot({ path: 'test-results/picker.png' })
    await note.fill('Переделай эту кнопку')
    await page.getByRole('button', { name: 'Отправить в чат' }).click()
    await expect(page.locator('.message.user').last()).toContainText('Переделай эту кнопку', { timeout: 15000 })
    await expect.poll(() => fixture.provider.prompts.length, { timeout: 15000 }).toBeGreaterThan(0)
    expect(JSON.stringify(fixture.provider.prompts.at(-1))).toContain('#buy')
  } finally {
    await fixture.close()
    server.close()
  }
})

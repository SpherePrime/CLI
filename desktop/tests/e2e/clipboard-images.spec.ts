import { test, expect } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

test('Ctrl V pastes and sends a screenshot without typed text', async () => {
  test.setTimeout(120000)
  const fixture = await launchWorkbench()
  const { app, page, provider } = fixture
  try {
    await app.evaluate(async ({ clipboard, nativeImage, ClipboardItem }) => {
      const previous = await clipboard.read()
      ;(globalThis as any).primeTestClipboard = await Promise.all(
        previous.map(
          async (item) =>
            new ClipboardItem(
              Object.fromEntries(
                await Promise.all(item.types.map(async (type) => [type, await item.getType(type)]))
              )
            )
        )
      )
      const png = nativeImage
        .createFromBitmap(Buffer.alloc(16 * 16 * 4, 180), { width: 16, height: 16 })
        .toPNG()
      await clipboard.write([new ClipboardItem({ 'image/png': new Blob([Uint8Array.from(png)]) })])
    })
    const editor = page.getByRole('textbox', { name: 'Сообщение Prime', exact: true })
    await editor.focus()
    await page.keyboard.press('Control+v')
    await expect(page.locator('.attachment-image img')).toBeVisible()
    await expect(editor).toHaveValue('')
    await expect(
      page.getByRole('button', { name: 'Отправить сообщение', exact: true })
    ).toBeEnabled()
    await page.screenshot({ path: 'test-results/clipboard-image.png' })
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect
      .poll(
        () =>
          provider.prompts.some((prompt) =>
            JSON.stringify(prompt).includes('data:image/png;base64,')
          ),
        { timeout: 30000 }
      )
      .toBe(true)
    await expect(page.locator('.attachment-chips')).toHaveCount(0)
    await expect(page.locator('.message.user .attachment-note img')).toBeVisible()
    expect(fixture.errors).toEqual([])
  } finally {
    await app.evaluate(async ({ clipboard }) => {
      const previous = (globalThis as any).primeTestClipboard
      if (previous?.length) await clipboard.write(previous)
      else clipboard.clear()
    })
    await fixture.close()
  }
})

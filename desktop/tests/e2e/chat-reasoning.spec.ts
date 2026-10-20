import { test, expect, type Page } from '@playwright/test'
import { launchWorkbench } from '../fixtures/workbench-app'

async function setLevel(page: Page, index: number) {
  await page.getByRole('button', { name: 'Модель и уровень размышлений', exact: true }).click()
  const slider = page.getByRole('slider', { name: 'Уровень размышлений', exact: true })
  await slider.press('Home')
  await expect(slider).toHaveAttribute('aria-valuenow', '0')
  for (let step = 1; step <= index; step++) {
    await expect(slider).toHaveAttribute('aria-disabled', 'false')
    await slider.press('ArrowRight')
    await expect(slider).toHaveAttribute('aria-valuenow', String(step))
  }
  await expect(slider).toHaveAttribute('aria-disabled', 'false')
  await page.keyboard.press('Escape')
}

test('keeps reasoning per chat across model switches, sends and restart', async () => {
  test.setTimeout(120000)
  const workbench = await launchWorkbench()
  let page = workbench.page
  try {
    await page.getByRole('button', { name: 'Новый чат', exact: true }).click()
    const project = await page.evaluate(async () =>
      (await window.prime.projects()).find((item) => item.kind === 'chat')!
    )
    const first = await page.evaluate(
      async (id) => (await window.prime.call(id, 'sessions'))[0],
      project.id
    )
    await page.evaluate(
      async ({ id, first, url }) => {
        await window.prime.call(id, 'renameSession', {
          sessionId: first.id,
          session: { ...first, title: 'First reasoning' }
        })
        await window.prime.call(id, 'providerSave', {
          scope: 0,
          originalId: 'fixture',
          provider: {
            id: 'fixture',
            name: 'Fixture',
            type: 'openai-compat',
            base_url: url,
            discover_models: false,
            models: [
              {
                id: 'desktop-fixture',
                name: 'Desktop Fixture',
                context_window: 128000,
                default_max_tokens: 8192
              },
              {
                id: 'secondary-fixture',
                name: 'Secondary Fixture',
                context_window: 128000,
                default_max_tokens: 8192
              }
            ]
          }
        })
      },
      { id: project.id, first, url: workbench.provider.url }
    )
    await setLevel(page, 3)
    await page.evaluate(() => window.dispatchEvent(new Event('prime:models')))
    await page.getByRole('option', { name: 'Secondary Fixture', exact: false }).click()
    await expect(page.locator('.model-control-level')).toHaveText('Высокий')
    const imageBytes = await workbench.app.evaluate(({ nativeImage }) =>
      nativeImage
        .createFromBitmap(Buffer.alloc(16, 255), { width: 2, height: 2 })
        .toPNG()
        .toString('base64')
    )
    await page
      .getByRole('textbox', { name: 'Сообщение Prime', exact: true })
      .evaluate((element, imageBytes) => {
        const bytes = Uint8Array.from(atob(imageBytes), (value) => value.charCodeAt(0))
        const data = new DataTransfer()
        data.items.add(new File([bytes], 'vision-fixture.png', { type: 'image/png' }))
        element.dispatchEvent(
          new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true })
        )
      }, imageBytes)
    await expect(page.locator('.attachment-chips')).toContainText('vision-fixture.png')
    const send = async (prompt: string, effort: string) => {
      await page.getByRole('textbox', { name: 'Сообщение Prime', exact: true }).fill(prompt)
      await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
      await expect
        .poll(
          () =>
            workbench.provider.prompts.find(
              (body) => body.stream && body.tools && JSON.stringify(body.messages).includes(prompt)
            )?.reasoning_effort
        )
        .toBe(effort)
      await expect(
        page.getByRole('button', { name: 'Отправить сообщение', exact: true })
      ).toBeVisible()
    }
    await send('first-reasoning-request', 'high')
    const delivered = workbench.provider.prompts.find(
      (body) => body.tools && JSON.stringify(body.messages).includes('first-reasoning-request')
    )
    expect(
      delivered.messages
        .flatMap((message: any) => (Array.isArray(message.content) ? message.content : []))
        .some(
          (part: any) =>
            part.type === 'image_url' && part.image_url?.url.startsWith('data:image/png;base64,')
        )
    ).toBe(true)
    await page.evaluate(
      ({ id, first }) =>
        window.prime.call(id, 'renameSession', {
          sessionId: first.id,
          session: { ...first, title: 'First reasoning' }
        }),
      { id: project.id, first }
    )
    await page.getByRole('button', { name: 'Новый чат', exact: true }).click()
    await expect(page.locator('.model-control-level')).toHaveText('Выключены')
    await setLevel(page, 1)
    await send('second-reasoning-request', 'low')
    await page.locator('.session-link').filter({ hasText: 'First reasoning' }).click()
    await expect(page.locator('.model-control-level')).toHaveText('Высокий')
    ;({ page } = await workbench.restart())
    await page.locator('.session-link').filter({ hasText: 'First reasoning' }).click()
    await expect(page.locator('.model-control-level')).toHaveText('Высокий')
    const modes = await page.evaluate(() =>
      window.prime.preferences().then((prefs) => prefs.reasoningBySession)
    )
    expect(modes?.[first.id]).toBe('high')
    expect(Object.values(modes || {})).toContain('low')
  } finally {
    await workbench.close()
  }
})

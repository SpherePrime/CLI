import { test, expect, _electron as electron } from '@playwright/test'
import { createServer } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

test('virtualizes a thousand variable-height messages and preserves both ends of history', async () => {
  test.setTimeout(90000)
  const server = await createServer({
    configFile: false,
    plugins: [react()],
    server: { host: '127.0.0.1', port: 0, watch: { ignored: ['**/out/**', '**/release/**'] } }
  })
  await server.listen()
  const url = server.resolvedUrls!.local[0] + 'tests/fixtures/history.html'
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    PRIME_HISTORY_TEST_URL: url
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({ args: [resolve('tests/fixtures/history-main.cjs')], env })
  try {
    const page = await desktop.firstWindow()
    await expect(page.getByText('Message 999', { exact: false })).toBeVisible()
    expect(await page.locator('.message').count()).toBeLessThan(40)
    await page.locator('.conversation').evaluate((element) => {
      element.scrollTop = 0
    })
    await expect(page.getByText('Message 0', { exact: false })).toBeVisible()
    await page.getByRole('button', { name: 'Append', exact: true }).click()
    await expect(page.getByText('Message 0', { exact: false })).toBeVisible()
    await page.getByRole('button', { name: 'Новые сообщения' }).click()
    await expect(page.getByText('Newest streamed answer')).toBeVisible()
    expect(await page.locator('.message').count()).toBeLessThan(40)
    const view = page.locator('.conversation')
    await page.getByRole('button', { name: 'Start stream', exact: true }).click()
    await view.hover()
    async function wheel(delta: number) {
      const received = view.evaluate(
        (element) =>
          new Promise<number>((resolve) => {
            element.addEventListener(
              'wheel',
              () => {
                const offset = element.scrollTop
                requestAnimationFrame(() => resolve(offset))
              },
              { once: true }
            )
          })
      )
      await page.mouse.wheel(0, delta)
      const offset = await received
      await view.evaluate((element, delta) => {
        element.scrollTop += delta
      }, delta)
      return offset
    }
    const beforeWheel = await wheel(-60)
    const readingPosition = await view.evaluate((element) => element.scrollTop)
    const tickBeforeWheel = Number(await page.getByTestId('stream-ticks').textContent())
    await expect
      .poll(async () => Number(await page.getByTestId('stream-ticks').textContent()))
      .toBeGreaterThan(tickBeforeWheel + 10)
    expect(await view.evaluate((element) => element.scrollTop)).toBeLessThanOrEqual(
      readingPosition + 2
    )
    await wheel(-1500)
    await expect
      .poll(() => view.evaluate((element) => element.scrollTop))
      .toBeLessThan(beforeWheel - 500)
    const firstVisibleIndex = () =>
      view.evaluate((element) => {
        const bounds = element.getBoundingClientRect()
        const row = [...element.querySelectorAll<HTMLElement>('[data-index]')].find((row) => {
          const box = row.getBoundingClientRect()
          return box.bottom > bounds.top + 1 && box.top < bounds.bottom
        })
        return Number(row?.dataset.index)
      })
    let middle = NaN
    await expect.poll(async () => (middle = await firstVisibleIndex())).toBeGreaterThanOrEqual(0)
    await wheel(500)
    await expect.poll(firstVisibleIndex).toBeGreaterThan(middle)
    await page.getByRole('button', { name: 'Новые сообщения' }).click()
    await expect
      .poll(() =>
        view.evaluate((element) => element.scrollHeight - element.scrollTop - element.clientHeight)
      )
      .toBeLessThan(3)
    await page.getByRole('button', { name: 'Stop stream', exact: true }).click()
    await page.getByRole('button', { name: 'Reasoning fixture', exact: true }).click()
    await page.getByText('Размышления', { exact: true }).click()
    await expect(page.locator('.reasoning')).toHaveAttribute('open', '')
    await page.locator('.conversation').evaluate((element) => {
      element.scrollTop = element.scrollHeight
    })
    await page.getByText('Размышления', { exact: true }).click()
    await expect(page.locator('.reasoning')).not.toHaveAttribute('open', '')
    await page.evaluate(
      () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
    )
    await expect(page.getByRole('button', { name: 'Новые сообщения' })).toHaveCount(0)
    await page.getByRole('button', { name: 'Tools fixture', exact: true }).click()
    await expect(page.locator('.tool-card')).toHaveCount(1)
    await expect(page.locator('.tool-card .tool-status')).toHaveAttribute('title', 'Выполняется')
    await page.getByRole('button', { name: 'Finish tool', exact: true }).click()
    await expect(page.locator('.tool-card')).toHaveCount(1)
    await expect(page.locator('.message.tool')).toHaveCount(0)
    await expect(page.locator('.tool-card .tool-status')).toHaveAttribute('title', 'Готово')
    await page.locator('.tool-card summary').click()
    await expect(page.locator('.tool-section pre').first()).toHaveText('printf "Hello, Prime!\\n"')
    await expect(page.locator('.tool-output pre')).toContainText('Hello, Prime!')
    await page
      .locator('.conversation-panel')
      .screenshot({ path: 'test-results/tool-card-component.png' })
  } finally {
    await desktop.close()
    await server.close()
  }
})

test('keeps an unresolved approval after a permission-start notification and snapshot refresh', async () => {
  const server = await createServer({
    configFile: false,
    plugins: [react()],
    server: { host: '127.0.0.1', port: 0, watch: { ignored: ['**/out/**', '**/release/**'] } }
  })
  await server.listen()
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    PRIME_HISTORY_TEST_URL: server.resolvedUrls!.local[0] + 'tests/fixtures/pending.html'
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({ args: [resolve('tests/fixtures/history-main.cjs')], env })
  try {
    const page = await desktop.firstWindow()
    await expect(page.getByText('Workspace ready')).toBeVisible()
    await page.getByRole('button', { name: 'Request permission', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Разрешить действие?' })).toBeVisible()
    await page.getByRole('button', { name: 'Resolve permission', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Разрешить действие?' })).toHaveCount(0)
    await page.getByRole('button', { name: 'Lost request with slow resources' }).click()
    await expect(page.getByText('Recovered without waiting for resources')).toBeVisible({
      timeout: 1800
    })
    await page.getByRole('button', { name: 'Cancel without resolution event' }).click()
    await expect(page.getByRole('heading', { name: 'Разрешить действие?' })).toHaveCount(0, {
      timeout: 1800
    })
  } finally {
    await desktop.close()
    await server.close()
  }
})

import { test, expect } from '@playwright/test'
import { createServer } from 'node:http'
import { launchWorkbench } from '../fixtures/workbench-app'

test('built-in browser keeps several tabs', async () => {
  test.setTimeout(90000)
  const server = createServer((request, response) => {
    response.setHeader('Content-Type', 'text/html; charset=utf-8')
    const name = request.url?.includes('second') ? 'Second' : 'First'
    response.end(`<title>${name}</title><h1>${name}</h1>`)
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const base = `http://127.0.0.1:${(server.address() as { port: number }).port}`
  const fixture = await launchWorkbench()
  const { page } = fixture
  const titles = async () => (await page.evaluate(() => window.prime.browserState())).tabs?.map((tab) => tab.title).join(',')
  try {
    await page.evaluate((url) => window.prime.browserNavigate(url), `${base}/first`)
    await page.evaluate((url) => window.prime.browserTab('new', undefined, url), `${base}/second`)
    await expect.poll(titles).toBe('First,Second')
    await expect(page.getByRole('tab', { name: /Second/ })).toHaveAttribute('aria-selected', 'true')
    await page.getByRole('tab', { name: /First/ }).click()
    await expect.poll(async () => (await page.evaluate(() => window.prime.browserState())).title).toBe('First')
    await page.screenshot({ path: 'test-results/browser-tabs.png' })
    await page.getByRole('button', { name: 'Закрыть вкладку First' }).click()
    await expect.poll(async () => (await page.evaluate(() => window.prime.browserState())).title).toBe('Second')
  } finally {
    await fixture.close()
    server.close()
  }
})

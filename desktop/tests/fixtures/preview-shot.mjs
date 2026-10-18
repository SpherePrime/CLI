import { _electron as electron } from '@playwright/test'
import { createServer } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

const output = process.argv[2] || 'test-results/preview.png'
const action = process.argv[3] || ''
const server = await createServer({
  configFile: false,
  plugins: [react()],
  logLevel: 'error',
  server: { host: '127.0.0.1', port: 0, watch: null }
})
await server.listen()
const env = Object.fromEntries(
  Object.entries(process.env).filter((entry) => entry[1] !== undefined)
)
env.PRIME_HISTORY_TEST_URL = server.resolvedUrls.local[0] + 'tests/fixtures/preview.html'
delete env.ELECTRON_RUN_AS_NODE
const desktop = await electron.launch({ args: [resolve('tests/fixtures/preview-main.cjs')], env })
try {
  const page = await desktop.firstWindow()
  page.on('pageerror', (error) => console.error('pageerror', error.message))
  page.on('console', (message) => {
    if (message.type() === 'error') console.error('console', message.text())
  })
  await page.setViewportSize({ width: 1000, height: 760 })
  await page.waitForSelector('.composer')
  if (action === 'todos') await page.locator('.todo-panel-head').click()
  if (action === 'context') await page.locator('.context-meter').click()
  if (action === 'reasoning') await page.locator('.model-control').click()
  await page.waitForTimeout(900)
  if (process.env.PREVIEW_PROBE)
    console.log(
      await page.evaluate((selector) => {
        const element = document.querySelector(selector)
        if (!element) return 'missing'
        const style = getComputedStyle(element)
        return JSON.stringify({
          text: element.textContent,
          opacity: style.opacity,
          color: style.color,
          display: style.display,
          height: element.getBoundingClientRect().height
        })
      }, process.env.PREVIEW_PROBE)
    )
  await page.screenshot({ path: output })
  if (process.env.PREVIEW_ELEMENT)
    await page
      .locator(process.env.PREVIEW_ELEMENT)
      .first()
      .screenshot({ path: output.replace('.png', '-element.png') })
  console.log(output)
} finally {
  await desktop.close()
  await server.close()
}

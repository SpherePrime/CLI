import { test, expect, _electron as electron } from '@playwright/test'
import { realpathSync } from 'node:fs'
import { mkdir, mkdtemp, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

test('built-in browser is shown to the user and driven by agent tools', async () => {
  test.setTimeout(120000)
  const project = realpathSync.native(await mkdtemp(join(tmpdir(), 'prime-browser-')))
  await mkdir(join(project, 'config'))
  await writeFile(
    join(project, 'config', 'prime.json'),
    JSON.stringify({
      options: {
        disable_default_providers: true,
        disable_provider_auto_update: true,
        disable_metrics: true
      },
      providers: {
        fixture: {
          id: 'fixture',
          name: 'Fixture',
          type: 'openai-compat',
          base_url: 'http://127.0.0.1:9/v1',
          api_key: 'fixture-key',
          discover_models: false,
          models: [
            {
              id: 'fixture-model',
              name: 'Fixture',
              context_window: 64000,
              default_max_tokens: 4096
            }
          ]
        }
      },
      models: {
        large: { provider: 'fixture', model: 'fixture-model' },
        small: { provider: 'fixture', model: 'fixture-model' }
      }
    })
  )
  const pagePath = join(project, 'page.html')
  await writeFile(
    pagePath,
    `<!doctype html><html><head><title>Fixture page</title></head><body style="font-family:sans-serif;padding:24px">
      <h1>Browser fixture</h1>
      <input id="name" placeholder="Your name">
      <button onclick="document.getElementById('out').textContent = 'Hello, ' + document.getElementById('name').value">Greet</button>
      <p id="out">waiting</p>
    </body></html>`
  )
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    PRIME_DESKTOP_TEST_PROJECT: project,
    PRIME_GLOBAL_CONFIG: join(project, 'config'),
    PRIME_GLOBAL_DATA: join(project, 'global-data'),
    PRIME_CACHE_DIR: join(project, 'cache'),
    XDG_CONFIG_HOME: join(project, 'xdg-config'),
    XDG_DATA_HOME: join(project, 'xdg-data')
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({
    args: [resolve('out/main/index.js'), `--user-data-dir=${join(project, 'electron-profile')}`],
    env
  })
  try {
    const page = await desktop.firstWindow()
    await expect
      .poll(() => page.evaluate(() => window.prime.info()), { timeout: 30000 })
      .toMatchObject({ runtime: 'ready' })
    await page.getByRole('button', { name: 'Браузер', exact: true }).click()
    await expect(page.getByText('Встроенный браузер', { exact: true })).toBeVisible()
    await page.getByRole('textbox', { name: 'Адрес страницы' }).fill(pagePath)
    await page.getByRole('textbox', { name: 'Адрес страницы' }).press('Enter')
    await expect(page.getByRole('textbox', { name: 'Адрес страницы' })).toHaveValue(
      pathToFileURL(pagePath).href
    )

    const endpoint = await desktop.evaluate(() => ({
      url: process.env.PRIME_DESKTOP_BROWSER_URL!,
      token: process.env.PRIME_DESKTOP_BROWSER_TOKEN!
    }))
    const call = async (name: string, args: Record<string, unknown> = {}) => {
      const response = await fetch(endpoint.url, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${endpoint.token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          jsonrpc: '2.0',
          id: 1,
          method: 'tools/call',
          params: { name, arguments: args }
        })
      })
      const body = await response.json()
      return body.result.content[0].text as string
    }
    const unauthorized = await fetch(endpoint.url, {
      method: 'POST',
      body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/list' })
    })
    expect(unauthorized.status).toBe(401)

    const snapshot = await call('browser_snapshot')
    expect(snapshot).toContain('Title: Fixture page')
    const inputRef = snapshot.match(/\[(\d+)\] input:text "Your name"/)![1]
    const buttonRef = snapshot.match(/\[(\d+)\] button "Greet"/)![1]
    await call('browser_type', { ref: Number(inputRef), text: 'Prime' })
    const after = await call('browser_click', { ref: Number(buttonRef) })
    expect(after).toContain('Hello, Prime')

    const projectId = await page.evaluate(async () => (await window.prime.projects())[0].id)
    await expect
      .poll(
        async () => {
          const states = await page.evaluate((id) => window.prime.call(id, 'mcp'), projectId)
          return states?.browser?.state
        },
        { timeout: 30000 }
      )
      .toBe('connected')

    await page.waitForTimeout(300)
    const image = await desktop.evaluate(async ({ BrowserWindow }) => {
      const window = BrowserWindow.getAllWindows()[0]
      return (await window.capturePage()).toPNG().toString('base64')
    })
    await writeFile('test-results/built-in-browser.png', Buffer.from(image, 'base64'))
    const view = await desktop.evaluate(async ({ BrowserWindow }) => {
      const child = BrowserWindow.getAllWindows()[0].contentView.children[0] as any
      return {
        bounds: child.getBounds(),
        image: (await child.webContents.capturePage()).toPNG().toString('base64')
      }
    })
    await writeFile('test-results/built-in-browser-view.png', Buffer.from(view.image, 'base64'))
    await writeFile('test-results/built-in-browser-view.json', JSON.stringify(view.bounds))
  } finally {
    await desktop.close()
  }
})

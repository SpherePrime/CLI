import { chromium } from '@playwright/test'
import { spawn } from 'node:child_process'
import { mkdtemp, mkdir, writeFile, appendFile, copyFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { resolve, join } from 'node:path'
import { startNativeProvider } from './native-provider.mjs'
import { createProject } from './native-ui-project.mjs'

export async function startUiAudit() {
  const root = await mkdtemp(join(tmpdir(), 'prime-ui-script-'))
  const output = resolve('tests/e2e/.workbench-ui-audit')
  await mkdir(output, { recursive: true })
  const project = await createProject(root)
  await writeFile(join(root, 'preferences.json'), JSON.stringify({ version: 1, preferences: { recent: [project], activeProject: join(root, 'Chats'), theme: 'dark', language: 'ru', drafts: {} } }))
  const provider = await startNativeProvider({ models: ['audit-alpha', 'audit-beta'], reply: input => JSON.stringify(input.messages || []).includes('UI_CHAT_B') ? 'UI_CHAT_B response' : 'UI_CHAT_A response' })
  await mkdir(join(root, 'config'), { recursive: true })
  await writeFile(join(root, 'config', 'prime.json'), JSON.stringify({ options: { disable_default_providers: true, disable_provider_auto_update: true, disable_metrics: true, disable_auto_summarize: true }, providers: { audit: { id: 'audit', name: 'UI Audit', type: 'openai-compat', base_url: provider.url, api_key: 'fixture', discover_models: false, models: ['alpha', 'beta'].map(id => ({ id: `audit-${id}`, name: `Audit ${id}`, can_reason: true, reasoning_levels: ['low','medium','high'], supports_attachments: true, context_window: 128000, default_max_tokens: 8192 })) } }, models: { large: { provider: 'audit', model: 'audit-alpha' }, small: { provider: 'audit', model: 'audit-alpha' } } }))
  let child, browser, page
  const delay = ms => new Promise(resolve => setTimeout(resolve, ms))
  const evidence = []
  let cycle = 0
  async function launch() {
    child = spawn(resolve('src-tauri/target/debug/prime-native.exe'), [], { windowsHide: true, env: { ...process.env, PRIME_NATIVE_TEST_DATA: root, WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS: '--remote-debugging-port=9241' } })
    child.stdout.on('data', data => appendFile(join(output, 'host.log'), data))
    child.stderr.on('data', data => appendFile(join(output, 'host.log'), data))
    for (let attempt = 0; attempt < 180; attempt++) {
      if (child.exitCode !== null) throw new Error(`Native app exited: ${child.exitCode}`)
      try { if ((await fetch('http://127.0.0.1:9241/json/version')).ok) break } catch {}
      await delay(200)
    }
    browser = await chromium.connectOverCDP('http://127.0.0.1:9241')
    await browser.contexts()[0].tracing.start({ screenshots: true, snapshots: true })
    cycle++
    page = browser.contexts().flatMap(context => context.pages()).find(item => !item.url().startsWith('devtools:'))
    await page.waitForURL(url => url.href.startsWith('http://127.0.0.1:5173'), { timeout: 30000 })
    page.on('pageerror', error => evidence.push({ error: error.message }))
    page.on('console', message => { if (message.type() === 'error') evidence.push({ console: message.text() }) })
    await page.getByRole('button', { name: 'Модель и уровень размышлений', exact: true }).waitFor()
    await page.waitForFunction(() => !document.querySelector('.startup-screen'))
    return page
  }
  async function step(name, action) {
    try {
      await action(page)
      await page.screenshot({ path: join(output, `${name}.png`) })
      evidence.push({ step: name, passed: true })
      console.log('PASS', name)
    } catch (error) {
      evidence.push({ step: name, passed: false, error: error.stack })
      await page.screenshot({ path: join(output, `${name}-failed.png`) }).catch(() => {})
      await writeFile(join(output, 'failure.html'), await page.content()).catch(() => {})
      throw error
    } finally {
      await writeFile(join(output, 'results.json'), JSON.stringify({ root, evidence, requests: provider.requests.map(request => ({ model: request.model, method: request.method, stream: request.stream, path: request.fixturePath })) }, null, 2))
    }
  }
  async function close() {
    if (browser) await browser.contexts()[0]?.tracing.stop({ path: join(output, `trace-${cycle}.zip`) }).catch(() => {})
    if (child?.exitCode === null) {
      await page.locator('.native-window-close').click()
      for (let attempt = 0; attempt < 100 && child.exitCode === null; attempt++) await delay(100)
      if (child.exitCode === null) throw new Error('Native shutdown timed out')
      await copyFile(join(root, 'preferences.json'), join(output, `preferences-after-close-${cycle}.json`))
    }
    await browser?.close().catch(() => {})
  }
  async function dispose() {
    await close().catch(() => { child?.kill() })
    provider.server.close()
  }
  const capture = name => page.screenshot({ path: join(output, `${name}.png`) })
  return { root, output, project, provider, launch, close, step, capture, dispose }
}

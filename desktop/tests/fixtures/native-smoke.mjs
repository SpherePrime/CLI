import assert from 'node:assert/strict'
import { chromium } from '@playwright/test'
import { spawn } from 'node:child_process'
import { mkdtemp, mkdir, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { startNativeProvider } from './native-provider.mjs'
import { nativeProcessFootprint } from './native-processes.mjs'

if (process.platform !== 'win32') throw new Error('This fixture uses the Windows WebView2 CDP adapter')
const root = await mkdtemp(join(tmpdir(), 'prime-native-ui-'))
const output = resolve('tests/e2e/.workbench-native')
await mkdir(output, { recursive: true })
const provider = await startNativeProvider()
const config = join(root, 'config')
await mkdir(config, { recursive: true })
await writeFile(join(config, 'prime.json'), JSON.stringify({ options: { disable_default_providers: true, disable_provider_auto_update: true, disable_metrics: true, disable_auto_summarize: true }, providers: { fixture: { id: 'fixture', name: 'Fixture', type: 'openai-compat', base_url: provider.url, api_key: 'fixture', discover_models: false, models: [{ id: 'native-fixture', name: 'Native Fixture', supports_attachments: true, context_window: 128000, default_max_tokens: 8192 }] } }, models: { large: { provider: 'fixture', model: 'native-fixture' }, small: { provider: 'fixture', model: 'native-fixture' } } }))
const port = Number(process.env.PRIME_NATIVE_CDP_PORT || 9238)
const child = spawn(resolve('src-tauri/target/debug/prime-native.exe'), [], { windowsHide: true, env: { ...process.env, PRIME_NATIVE_TEST_DATA: root, WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS: `--remote-debugging-port=${port}` } })
let log = ''
child.stdout.on('data', (chunk) => { log += chunk })
child.stderr.on('data', (chunk) => { log += chunk })
let browser
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
try {
  let ready = false
  for (let attempt = 0; attempt < 200; attempt++) {
    if (child.exitCode !== null) throw new Error(`Native app exited: ${log}`)
    try { if ((await fetch(`http://127.0.0.1:${port}/json/version`)).ok) { ready = true; break } } catch {}
    await delay(250)
  }
  assert(ready, `WebView2 did not start: ${log}`)
  browser = await chromium.connectOverCDP(`http://127.0.0.1:${port}`)
  const page = browser.contexts().flatMap((context) => context.pages()).find((page) => !page.url().startsWith('devtools:'))
  assert(page, 'Main webview is missing')
  await page.waitForURL((url) => url.href.startsWith('http://127.0.0.1:5173'), { timeout: 30000 })
  await page.waitForFunction(() => Boolean(window.prime))
  let info
  for (let attempt = 0; attempt < 150; attempt++) {
    info = await page.evaluate(() => window.prime.info())
    if (info.runtime !== 'starting') break
    await delay(300)
  }
  assert.equal(info.runtime, 'ready', JSON.stringify(info))
  const project = await page.evaluate(() => window.prime.openChat())
  const session = await page.evaluate((id) => window.prime.call(id, 'createSession', { title: 'Native smoke' }), project.id)
  await page.evaluate(() => { window.__nativeEvents = []; window.prime.onEvent((event) => { if (event.type === 'message' || event.type === 'run_complete') window.__nativeEvents.push(event.type) }) })
  await page.evaluate((id) => window.prime.call(id, 'init'), project.id)
  const images = await page.evaluate(() => window.prime.importImages([{ name: 'fixture.png', data: Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a5V8AAAAASUVORK5CYII='), (character) => character.charCodeAt(0)) }]))
  await page.evaluate(({ id, sessionId, attachmentIds }) => window.prime.call(id, 'send', { sessionId, attachmentIds, runId: crypto.randomUUID(), prompt: 'Describe the image briefly', reasoningMode: 'off' }), { id: project.id, sessionId: session.id, attachmentIds: images.map((image) => image.id) })
  await page.waitForFunction(() => window.__nativeEvents.includes('run_complete'), null, { timeout: 30000 })
  assert(provider.requests.some((request) => JSON.stringify(request.messages).includes('image_url')), 'Image did not reach the provider')
  await page.evaluate((ids) => window.prime.releaseAttachments(ids), images.map((image) => image.id))
  const terminal = await page.evaluate((id) => window.prime.terminal(id, { action: 'create', options: { title: 'Native fixture' } }), project.id)
  assert.equal(terminal.status, 'running', 'Native PTY did not start')
  await page.evaluate(({ workspace, id }) => window.prime.terminal(workspace, { action: 'write', id, data: "Write-Output ('NATIVE_UI_' + 'PTY_OK'); exit 0\r" }), { workspace: project.id, id: terminal.id })
  let terminalOutput = ''
  for (let attempt = 0; attempt < 100; attempt++) {
    const result = await page.evaluate(({ workspace, id }) => window.prime.terminal(workspace, { action: 'read', id }), { workspace: project.id, id: terminal.id })
    terminalOutput = result.output
    if (terminalOutput.includes('NATIVE_UI_PTY_OK') && result.terminal.status === 'exited') break
    await delay(100)
  }
  assert(terminalOutput.includes('NATIVE_UI_PTY_OK'), `Native terminal did not execute the command: ${JSON.stringify(terminalOutput)}`)
  await page.evaluate(({ workspace, id }) => window.prime.terminal(workspace, { action: 'close', id }), { workspace: project.id, id: terminal.id })
  await page.reload()
  await page.waitForFunction(() => document.body.innerText.includes('Native engine response'), null, { timeout: 30000 })
  await page.screenshot({ path: join(output, 'chat.png') })
  const footprint = await nativeProcessFootprint(child.pid)
  assert(!footprint.processes.some((process) => /^prime(?:\.exe)?$/i.test(process.Name)), 'Unexpected local CLI process')
  await writeFile(join(output, 'result.json'), JSON.stringify({ root, info, project, session, imageDelivered: true, footprint }, null, 2))
  await page.evaluate(() => window.__TAURI__.core.invoke('desktop_window', { action: 'close' }))
  for (let attempt = 0; attempt < 100 && child.exitCode === null; attempt++) await delay(100)
  assert.notEqual(child.exitCode, null, 'Native app did not shut down')
  console.log('Native UI, image delivery, history reload, PTY and graceful shutdown passed')
} finally {
  if (browser) await browser.close().catch(() => {})
  if (child.exitCode === null) child.kill()
  provider.server.close()
  await writeFile(join(output, 'host.log'), log)
}

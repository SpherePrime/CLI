import { app, BrowserWindow } from 'electron'
import { WindowViewer } from '../../src/main/computer/viewer'
import { listWindows, windowInfo } from '../../src/main/computer/win32'
const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))
const assert = (condition: unknown, message: string) => {
  if (!condition) throw new Error(message)
}
async function main() {
  await app.whenReady()
  const fixture = new BrowserWindow({
    width: 540,
    height: 440,
    title: 'Prime Window Control Fixture',
    show: true,
    webPreferences: { backgroundThrottling: false }
  })
  const other = new BrowserWindow({
    width: 240,
    height: 180,
    title: 'Prime Window Guard Fixture',
    parent: fixture,
    show: false
  })
  const viewer = new WindowViewer({ emit() {}, busySessions: async () => ['fixture-chat'] })
  try {
    await fixture.loadURL(
      'data:text/html,' +
        encodeURIComponent(
          '<title>Prime Window Control Fixture</title><body style="background:#163429;color:white;height:2000px"><button id="button" onclick="window.clicks=(window.clicks||0)+1">Fixture button</button><input id="input" aria-label="Fixture input"><div style="height:1800px">Scroll fixture</div></body>'
        )
    )
    fixture.setTitle('Prime Window Control Fixture')
    await delay(350)
    const hwnd = fixture.getNativeWindowHandle().readBigUInt64LE().toString()
    const owner = other.getNativeWindowHandle().readBigUInt64LE().toString()
    const listing = listWindows()
    assert(
      listing.some((item) => item.hwnd === hwnd),
      'Native listing omitted fixture'
    )
    const capture = await viewer.capture(hwnd)
    assert(capture.png.length > 500, 'Capture is empty')
    const run = async (name: string, args = {}) =>
      viewer
        .tools()
        .find((tool) => tool.name === name)!
        .run(args, { workspaceId: 'fixture-project', sessionId: 'fixture-chat' })
    let denied = false
    try {
      await run('window_action', { action: 'type', text: 'denied' })
    } catch {
      denied = true
    }
    assert(denied, 'Missing grant accepted input')
    const listed = (await run('window_list'))[0]
    const selected = JSON.parse(listed.type === 'text' ? listed.text : '').find(
      (item: { hwnd: string }) => item.hwnd === hwnd
    )
    await run('window_request_access', {
      hwnd,
      pid: selected.pid,
      identity: selected.identity,
      title: selected.title,
      mode: 'control'
    })
    const granted = await viewer.handle('fixture-project', { action: 'state' })
    assert(
      granted.share?.hwnd === hwnd && !granted.request,
      'Approved execution required another confirmation'
    )
    await run('window_action', { action: 'focus' })
    const point = async (id: string) => {
      const rect = await fixture.webContents.executeJavaScript(
        `(()=>{const r=document.getElementById(${JSON.stringify(id)}).getBoundingClientRect(); return {x:r.x+r.width/2,y:r.y+r.height/2}})()`
      )
      const content = fixture.getContentBounds()
      const bounds = windowInfo(hwnd)!.bounds
      return {
        x: Math.round(content.x + rect.x - bounds.x),
        y: Math.round(content.y + rect.y - bounds.y)
      }
    }
    await run('window_action', { action: 'click', ...(await point('button')) })
    await delay(150)
    assert(
      (await fixture.webContents.executeJavaScript('window.clicks')) === 1,
      'Native click did not reach fixture'
    )
    await run('window_action', { action: 'click', ...(await point('input')) })
    for (
      let attempt = 0;
      attempt < 40 &&
      !(await fixture.webContents.executeJavaScript(
        'document.activeElement.id === ' + JSON.stringify('input')
      ));
      attempt++
    )
      await delay(50)
    assert(
      await fixture.webContents.executeJavaScript(
        'document.activeElement.id === ' + JSON.stringify('input')
      ),
      'Native click did not focus the input'
    )
    await run('window_action', { action: 'type', text: 'Привет fixture' })
    await delay(150)
    assert(
      (await fixture.webContents.executeJavaScript('document.getElementById("input").value')) ===
        'Привет fixture',
      'Unicode typing failed'
    )
    await run('window_action', { action: 'press_key', key: 'CTRL+A' })
    await run('window_action', { action: 'type', text: 'updated' })
    await delay(150)
    assert(
      (await fixture.webContents.executeJavaScript('document.getElementById("input").value')) ===
        'updated',
      'Key shortcut failed'
    )
    await run('window_action', { action: 'scroll', x: 300, y: 300, delta: -480 })
    await delay(300)
    assert((await fixture.webContents.executeJavaScript('window.scrollY')) > 0, 'Scrolling failed')
    let shortcutDenied = false
    try {
      await run('window_action', { action: 'press_key', key: 'ALT+TAB' })
    } catch {
      shortcutDenied = true
    }
    assert(shortcutDenied, 'Desktop-wide shortcut was accepted')
    other.show()
    assert(
      listWindows().some((item) => item.hwnd === owner),
      'Owned window omitted from listing'
    )
    other.focus()
    await delay(100)
    denied = false
    try {
      await run('window_action', { action: 'type', text: 'forbidden' })
    } catch {
      denied = true
    }
    assert(denied, 'Wrong foreground accepted input')
    await viewer.handle('fixture-project', { action: 'revoke' })
    denied = false
    try {
      await run('window_action', { action: 'focus' })
    } catch {
      denied = true
    }
    assert(denied, 'Revoked grant accepted input')
    console.log(
      JSON.stringify({
        listing: true,
        captureBytes: capture.png.length,
        click: true,
        unicode: true,
        key: true,
        scroll: true,
        foregroundGuard: true,
        revoke: true,
        otherFixture: !!owner
      })
    )
    viewer.dispose()
    fixture.destroy()
    other.destroy()
    app.exit(0)
  } catch (error) {
    console.error(error)
    fixture.destroy()
    other.destroy()
    app.exit(1)
  }
}
void main().catch((error) => {
  console.error(error)
  app.exit(1)
})

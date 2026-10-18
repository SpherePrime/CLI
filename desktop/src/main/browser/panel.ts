import { BrowserWindow, WebContentsView, session, type WebContents } from 'electron'

export type BrowserBounds = { x: number; y: number; width: number; height: number }

export type BrowserState = {
  url: string
  title: string
  loading: boolean
  canGoBack: boolean
  canGoForward: boolean
}

const partition = 'persist:prime-browser'
const allowedProtocols = ['http:', 'https:', 'file:', 'about:']

export function normalizeBrowserUrl(value: string): string {
  const input = value.trim()
  if (!input) return 'about:blank'
  if (/^[a-z]:[\\/]/i.test(input)) return 'file:///' + input.replaceAll('\\', '/')
  if (/^[a-z][a-z0-9+.-]*:/i.test(input)) {
    const url = new URL(input)
    if (!allowedProtocols.includes(url.protocol))
      throw new Error(`Протокол ${url.protocol} не поддерживается`)
    return url.href
  }
  if (/^(localhost|127\.0\.0\.1|\[::1\])(:\d+)?(\/|$)/i.test(input)) return 'http://' + input
  if (/^[^\s]+\.[^\s]{2,}(\/.*)?$/.test(input)) return 'https://' + input
  return 'https://duckduckgo.com/?q=' + encodeURIComponent(input)
}

export class BrowserPanel {
  private view: WebContentsView | null = null
  private attached = false
  private bounds: BrowserBounds = { x: 0, y: 0, width: 0, height: 0 }

  constructor(
    private readonly window: () => BrowserWindow | null,
    private readonly publish: (event: any) => void
  ) {}

  get contents(): WebContents {
    return this.ensure().webContents
  }

  state(): BrowserState {
    const contents = this.view?.webContents
    if (!contents || contents.isDestroyed())
      return { url: '', title: '', loading: false, canGoBack: false, canGoForward: false }
    return {
      url: contents.getURL(),
      title: contents.getTitle(),
      loading: contents.isLoading(),
      canGoBack: contents.navigationHistory.canGoBack(),
      canGoForward: contents.navigationHistory.canGoForward()
    }
  }

  show(bounds: BrowserBounds) {
    const window = this.window()
    if (!window || window.isDestroyed()) return
    const view = this.ensure()
    this.bounds = {
      x: Math.round(bounds.x),
      y: Math.round(bounds.y),
      width: Math.max(0, Math.round(bounds.width)),
      height: Math.max(0, Math.round(bounds.height))
    }
    view.setBounds(this.bounds)
    if (!this.attached) {
      window.contentView.addChildView(view)
      this.attached = true
    }
  }

  hide() {
    const window = this.window()
    if (this.view && this.attached && window && !window.isDestroyed())
      window.contentView.removeChildView(this.view)
    this.attached = false
  }

  async navigate(value: string) {
    const url = normalizeBrowserUrl(value)
    this.publish({ type: 'browser', payload: { action: 'activate' } })
    await this.contents.loadURL(url).catch((error) => {
      if (!String(error?.message).includes('ERR_ABORTED')) throw error
    })
    return this.state()
  }

  back() {
    if (this.view?.webContents.navigationHistory.canGoBack())
      this.view.webContents.navigationHistory.goBack()
  }

  forward() {
    if (this.view?.webContents.navigationHistory.canGoForward())
      this.view.webContents.navigationHistory.goForward()
  }

  reload() {
    const contents = this.view?.webContents
    if (!contents) return
    if (contents.isLoading()) contents.stop()
    else contents.reload()
  }

  destroy() {
    this.hide()
    if (this.view && !this.view.webContents.isDestroyed()) this.view.webContents.close()
    this.view = null
  }

  private ensure(): WebContentsView {
    if (this.view && !this.view.webContents.isDestroyed()) return this.view
    const browserSession = session.fromPartition(partition)
    browserSession.setPermissionRequestHandler((_contents, _permission, callback) =>
      callback(false)
    )
    const view = new WebContentsView({
      webPreferences: {
        partition,
        contextIsolation: true,
        nodeIntegration: false,
        sandbox: true,
        webSecurity: true
      }
    })
    view.setBackgroundColor('#ffffff')
    const contents = view.webContents
    contents.setWindowOpenHandler(({ url }) => {
      try {
        void contents.loadURL(normalizeBrowserUrl(url))
      } catch {}
      return { action: 'deny' }
    })
    contents.on('will-navigate', (event, url) => {
      try {
        normalizeBrowserUrl(url)
      } catch {
        event.preventDefault()
      }
    })
    const update = () =>
      this.publish({ type: 'browser', payload: { action: 'state', ...this.state() } })
    for (const name of [
      'did-start-loading',
      'did-stop-loading',
      'did-navigate',
      'did-navigate-in-page',
      'page-title-updated'
    ] as const)
      contents.on(name as any, update)
    this.view = view
    return view
  }
}

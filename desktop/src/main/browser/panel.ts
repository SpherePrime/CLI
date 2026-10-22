import { BrowserWindow, WebContentsView, session, type NativeImage, type WebContents } from 'electron'
import { randomUUID } from 'node:crypto'
import { TabThrottle } from './tabThrottle'

export type BrowserBounds = { x: number; y: number; width: number; height: number }

export type BrowserLog = {
  level: 'error' | 'warning' | 'info'
  message: string
  source?: string
  line?: number
  url: string
  time: number
}

export type BrowserTabState = { id: string; url: string; title: string; loading: boolean }

export type BrowserState = {
  url: string
  title: string
  loading: boolean
  canGoBack: boolean
  canGoForward: boolean
  activeTab?: string
  tabs?: BrowserTabState[]
}

const partition = 'persist:prime-browser'
const allowedProtocols = ['http:', 'https:', 'file:', 'about:']
const maxTabs = 12

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

type Tab = { id: string; view: WebContentsView }

export class BrowserPanel {
  private tabs: Tab[] = []
  private active?: string
  private attached?: WebContentsView
  private visible = false
  private bounds: BrowserBounds = { x: 0, y: 0, width: 1280, height: 720 }
  private entries: BrowserLog[] = []
  private captures = new Map<string, Promise<NativeImage>>()
  private throttle = new TabThrottle(
    (contents) => this.visible && this.attached?.webContents === contents
  )

  constructor(
    private readonly window: () => BrowserWindow | null,
    private readonly publish: (event: any) => void
  ) {}

  get contents(): WebContents {
    return this.getTab().contents
  }

  getTab(tabId?: string): { id: string; contents: WebContents } {
    if (tabId === undefined) this.ensure()
    const tab = this.tabs.find((item) => item.id === (tabId ?? this.active))
    if (!tab || tab.view.webContents.isDestroyed()) throw new Error('Вкладка не найдена')
    this.throttle.touch(tab.view.webContents)
    return { id: tab.id, contents: tab.view.webContents }
  }

  async captureTab(tabId?: string): Promise<NativeImage> {
    const target = this.getTab(tabId)
    const pending = this.captures.get(target.id)
    if (pending) return pending
    const tab = this.tabs.find((item) => item.id === target.id)!
    const capture = this.captureView(tab)
    this.captures.set(target.id, capture)
    try {
      return await capture
    } finally {
      this.captures.delete(target.id)
    }
  }

  private async captureView(tab: Tab): Promise<NativeImage> {
    const window = this.window()
    if (!window || window.isDestroyed()) throw new Error('Окно браузера недоступно')
    const temporary = this.attached !== tab.view
    if (temporary) window.contentView.addChildView(tab.view, 0)
    let timer: ReturnType<typeof setTimeout> | undefined
    try {
      return await Promise.race([
        tab.view.webContents.capturePage(undefined, { stayHidden: true, stayAwake: true }),
        new Promise<never>((_resolve, reject) => {
          timer = setTimeout(() => reject(new Error('Не удалось получить снимок вкладки за 5 секунд')), 5000)
        })
      ])
    } finally {
      if (timer) clearTimeout(timer)
      if (temporary && this.attached !== tab.view && !window.isDestroyed())
        window.contentView.removeChildView(tab.view)
    }
  }

  private current(): Tab | undefined {
    const tab = this.tabs.find((item) => item.id === this.active)
    return tab && !tab.view.webContents.isDestroyed() ? tab : undefined
  }

  private tabState(tab: Tab): BrowserTabState {
    const contents = tab.view.webContents
    return { id: tab.id, url: contents.getURL(), title: contents.getTitle(), loading: contents.isLoading() }
  }

  state(): BrowserState {
    const tabs = this.tabs.filter((tab) => !tab.view.webContents.isDestroyed()).map((tab) => this.tabState(tab))
    const contents = this.current()?.view.webContents
    if (!contents)
      return { url: '', title: '', loading: false, canGoBack: false, canGoForward: false, tabs, activeTab: this.active }
    return {
      url: contents.getURL(),
      title: contents.getTitle(),
      loading: contents.isLoading(),
      canGoBack: contents.navigationHistory.canGoBack(),
      canGoForward: contents.navigationHistory.canGoForward(),
      activeTab: this.active,
      tabs
    }
  }

  logs(): BrowserLog[] {
    return [...this.entries]
  }

  clearLogs() {
    this.entries = []
  }

  private log(entry: Omit<BrowserLog, 'time' | 'url'>, contents: WebContents) {
    const url = contents.isDestroyed() ? '' : contents.getURL()
    this.entries.push({ ...entry, message: entry.message.slice(0, 4000), url, time: Date.now() })
    if (this.entries.length > 200) this.entries.splice(0, this.entries.length - 200)
    this.publish({ type: 'browser', payload: { action: 'log' } })
  }

  private update() {
    this.publish({ type: 'browser', payload: { action: 'state', ...this.state() } })
  }

  private attach() {
    for (const tab of this.tabs)
      if (!tab.view.webContents.isDestroyed()) tab.view.setBounds(this.bounds)
    const window = this.window()
    if (!window || window.isDestroyed()) return
    const view = this.current()?.view
    if (this.attached && (this.attached !== view || !this.visible)) {
      window.contentView.removeChildView(this.attached)
      this.attached = undefined
    }
    if (!this.visible || !view) return
    if (this.attached !== view) {
      window.contentView.addChildView(view)
      this.attached = view
    }
  }

  show(bounds: BrowserBounds) {
    this.ensure()
    this.bounds = {
      x: Math.round(bounds.x),
      y: Math.round(bounds.y),
      width: Math.max(0, Math.round(bounds.width)),
      height: Math.max(0, Math.round(bounds.height))
    }
    this.visible = true
    this.attach()
  }

  hide() {
    this.visible = false
    this.attach()
  }

  newTab(url?: string) {
    const target = url ? normalizeBrowserUrl(url) : undefined
    const tab = this.createTab(true)
    if (target) void this.loadTab(tab, target).catch(() => {})
    return this.state()
  }

  async openTab(value = 'about:blank', activate = false): Promise<BrowserTabState> {
    const url = normalizeBrowserUrl(value)
    const tab = this.createTab(activate)
    await this.loadTab(tab, url)
    return this.tabState(tab)
  }

  private createTab(activate: boolean): Tab {
    const tab = this.create()
    if (activate || !this.current()) this.active = tab.id
    this.attach()
    this.update()
    return tab
  }

  private async loadTab(tab: Tab, url: string) {
    await tab.view.webContents.loadURL(url).catch((error) => {
      if (!String(error?.message).includes('ERR_ABORTED')) throw error
    })
  }

  selectTab(id: string) {
    this.getTab(id)
    this.active = id
    this.attach()
    this.update()
    return this.state()
  }

  closeTab(id: string) {
    this.getTab(id)
    const index = this.tabs.findIndex((tab) => tab.id === id)
    const [tab] = this.tabs.splice(index, 1)
    if (this.attached === tab.view) {
      this.window()?.contentView.removeChildView(tab.view)
      this.attached = undefined
    }
    this.throttle.forget(tab.view.webContents)
    if (!tab.view.webContents.isDestroyed()) tab.view.webContents.close()
    if (this.active === id) this.active = (this.tabs[index] ?? this.tabs[index - 1])?.id
    this.attach()
    this.update()
    return this.state()
  }

  reveal(agent = false, width?: number) {
    this.publish({
      type: 'browser',
      payload: { action: 'activate', agent, ...(width ? { width: Math.round(width) } : {}) }
    })
  }

  async navigate(value: string, agent = false, width?: number, tabId?: string) {
    const url = normalizeBrowserUrl(value)
    const { contents } = this.getTab(tabId)
    this.reveal(agent, width)
    await contents.loadURL(url).catch((error) => {
      if (!String(error?.message).includes('ERR_ABORTED')) throw error
    })
    return this.state()
  }

  back(tabId?: string) {
    const contents = tabId === undefined ? this.current()?.view.webContents : this.getTab(tabId).contents
    if (contents) this.throttle.touch(contents)
    if (contents?.navigationHistory.canGoBack()) contents.navigationHistory.goBack()
  }

  forward(tabId?: string) {
    const contents = tabId === undefined ? this.current()?.view.webContents : this.getTab(tabId).contents
    if (contents) this.throttle.touch(contents)
    if (contents?.navigationHistory.canGoForward()) contents.navigationHistory.goForward()
  }

  reload(tabId?: string) {
    const contents = tabId === undefined ? this.current()?.view.webContents : this.getTab(tabId).contents
    if (contents) this.throttle.touch(contents)
    if (!contents) return
    if (contents.isLoading()) contents.stop()
    else contents.reload()
  }

  destroy() {
    this.hide()
    this.throttle.dispose()
    for (const tab of this.tabs) if (!tab.view.webContents.isDestroyed()) tab.view.webContents.close()
    this.tabs = []
    this.active = undefined
  }

  private ensure(): WebContentsView {
    const current = this.current()
    if (current) return current.view
    this.tabs = this.tabs.filter((tab) => !tab.view.webContents.isDestroyed())
    const tab = this.tabs[0] ?? this.create()
    this.active = tab.id
    return tab.view
  }

  private create(): Tab {
    this.tabs = this.tabs.filter((tab) => !tab.view.webContents.isDestroyed())
    if (this.tabs.length >= maxTabs) throw new Error(`Можно открыть не больше ${maxTabs} вкладок`)
    const browserSession = session.fromPartition(partition)
    browserSession.setPermissionRequestHandler((_contents, _permission, callback) => callback(false))
    const view = new WebContentsView({
      webPreferences: {
        partition,
        contextIsolation: true,
        nodeIntegration: false,
        sandbox: true,
        webSecurity: true,
        backgroundThrottling: false
      }
    })
    view.setBackgroundColor('#ffffff')
    view.setBounds(this.bounds)
    const tab: Tab = { id: randomUUID(), view }
    const contents = view.webContents
    this.throttle.touch(contents)
    contents.setWindowOpenHandler(({ url }) => {
      try {
        const target = normalizeBrowserUrl(url)
        if (this.tabs.length < maxTabs) void this.openTab(target, this.active === tab.id).catch(() => {})
        else void contents.loadURL(target)
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
    contents.on('console-message', (event: any) => {
      const level = event.level === 'error' ? 'error' : event.level === 'warning' ? 'warning' : undefined
      if (level && !String(event.message ?? '').includes('Electron Security Warning'))
        this.log({ level, message: String(event.message ?? ''), source: event.sourceId, line: event.lineNumber }, contents)
    })
    contents.on('did-fail-load', (_event, code, description, url, mainFrame) => {
      if (code !== -3)
        this.log({ level: 'error', message: `${description} (${code})${mainFrame ? '' : ' subframe'} ${url}` }, contents)
    })
    contents.on('render-process-gone', (_event, details) =>
      this.log({ level: 'error', message: `Renderer process gone: ${details.reason}` }, contents)
    )
    for (const name of ['did-start-loading', 'did-stop-loading', 'did-navigate', 'did-navigate-in-page', 'page-title-updated'] as const)
      contents.on(name as any, () => this.update())
    this.tabs.push(tab)
    return tab
  }
}

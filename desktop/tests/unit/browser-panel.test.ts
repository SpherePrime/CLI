import { describe, expect, it, vi } from 'vitest'
import { BrowserPanel } from '../../src/main/browser/panel'

vi.mock('electron', () => ({
  session: { fromPartition: () => ({ setPermissionRequestHandler() {} }) },
  WebContentsView: class {
    bounds: any
    constructor(public options: any) {}
    setBounds(bounds: any) { this.bounds = bounds }
    setBackgroundColor() {}
    webContents = {
      url: '',
      destroyed: false,
      history: [] as string[],
      index: -1,
      windowOpen: undefined as any,
      isDestroyed() { return this.destroyed },
      getURL() { return this.url },
      getTitle() { return this.url },
      isLoading: () => false,
      on() {},
      setWindowOpenHandler(handler: any) { this.windowOpen = handler },
      async loadURL(url: string) {
        this.url = url
        this.history.push(url)
        this.index = this.history.length - 1
      },
      close() { this.destroyed = true },
      reload() {},
      stop() {},
      navigationHistory: {
        canGoBack: () => false,
        canGoForward: () => false,
        goBack() {},
        goForward() {}
      }
    }
  }
}))

describe('browser panel targets', () => {
  it('opens and navigates background tabs without changing the visible tab', async () => {
    const panel = new BrowserPanel(() => null, () => {})
    const first = panel.newTab('https://first.example')
    const second = await panel.openTab('https://second.example')
    expect(panel.state().activeTab).toBe(first.activeTab)
    expect(second.url).toBe('https://second.example/')
    await panel.navigate('https://updated.example', false, undefined, second.id)
    expect(panel.state().url).toBe('https://first.example/')
    expect(panel.state().tabs?.find((tab) => tab.id === second.id)?.url).toBe('https://updated.example/')
  })

  it('pins a target across active changes and rejects closed or unknown targets', async () => {
    const panel = new BrowserPanel(() => null, () => {})
    const first = panel.getTab()
    expect((panel as any).tabs[0].view.bounds).toEqual({ x: 0, y: 0, width: 1280, height: 720 })
    const second = await panel.openTab('https://second.example', true)
    expect(first.id).not.toBe(second.id)
    expect(first.contents).not.toBe(panel.getTab().contents)
    panel.closeTab(first.id)
    expect(() => panel.getTab(first.id)).toThrow()
    expect(() => panel.closeTab(first.id)).toThrow()
    expect(() => panel.selectTab(first.id)).toThrow()
    await expect(panel.navigate('https://wrong.example', false, undefined, first.id)).rejects.toThrow()
    expect(() => panel.back('missing')).toThrow()
    expect(() => panel.forward('missing')).toThrow()
    expect(() => panel.reload('missing')).toThrow()
    expect(panel.state().url).toBe('https://second.example/')
  })

  it('sizes background views and keeps their popup tabs in the background', async () => {
    const children = new Set<any>()
    const panel = new BrowserPanel(() => ({
      isDestroyed: () => false,
      contentView: {
        addChildView: (view: any) => children.add(view),
        removeChildView: (view: any) => children.delete(view)
      }
    }) as any, () => {})
    const first = await panel.openTab('https://first.example')
    const second = await panel.openTab('https://second.example')
    panel.show({ x: 5, y: 10, width: 800, height: 600 })
    const tabs = (panel as any).tabs
    expect(tabs[1].view.bounds).toEqual({ x: 5, y: 10, width: 800, height: 600 })
    expect(tabs[1].view.options.webPreferences.backgroundThrottling).toBe(false)
    expect(children.size).toBe(1)
    ;(panel.getTab(second.id).contents as any).windowOpen({ url: 'https://popup.example' })
    await Promise.resolve()
    expect(panel.state().activeTab).toBe(first.id)
    expect(panel.state().tabs?.map((tab) => tab.url)).toEqual([
      'https://first.example/', 'https://second.example/', 'https://popup.example/'
    ])
  })

  it('temporarily renders a background capture behind the application and cleans up', async () => {
    const mainView = {}
    const children: any[] = [mainView]
    const panel = new BrowserPanel(() => ({
      isDestroyed: () => false,
      contentView: {
        addChildView: (view: any, index = children.length) => children.splice(index, 0, view),
        removeChildView: (view: any) => children.splice(children.indexOf(view), 1)
      }
    }) as any, () => {})
    const first = await panel.openTab('https://first.example')
    const second = await panel.openTab('https://second.example')
    let finishCapture!: (value: any) => void
    ;(panel.getTab(second.id).contents as any).capturePage = () => new Promise((resolve) => { finishCapture = resolve })
    const capture = panel.captureTab(second.id)
    const simultaneousCapture = panel.captureTab(second.id)
    expect(children[0]).toBe((panel as any).tabs[1].view)
    expect(children[1]).toBe(mainView)
    expect(children).toHaveLength(2)
    expect(panel.state().activeTab).toBe(first.id)
    const image = { image: 'background' }
    finishCapture(image)
    expect(await capture).toBe(image)
    expect(await simultaneousCapture).toBe(image)
    expect(children).toEqual([mainView])
  })
})

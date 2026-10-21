import type { BrowserPanel } from './panel'
import { browserTarget, tabIdProperty, type BrowserTarget } from './tabTarget'
import { browserTabTools } from './tabTools'

export type ToolContent =
  { type: 'text'; text: string } | { type: 'image'; data: string; mimeType: string }

export type Tool = {
  name: string
  description: string
  inputSchema: Record<string, unknown>
  run: (panel: BrowserPanel, args: any) => Promise<ToolContent[]>
}

const snapshotScript = `(() => {
  const selector = 'a[href],button,input,select,textarea,summary,[role=button],[role=link],[role=checkbox],[role=tab],[role=menuitem],[contenteditable=true],[onclick]'
  const visible = (element) => {
    const rect = element.getBoundingClientRect()
    const style = getComputedStyle(element)
    return rect.width > 0 && rect.height > 0 && style.visibility !== 'hidden' && style.display !== 'none'
  }
  document.querySelectorAll('[data-prime-ref]').forEach((element) => element.removeAttribute('data-prime-ref'))
  const elements = []
  let ref = 0
  for (const element of document.querySelectorAll(selector)) {
    if (!visible(element) || elements.length >= 300) continue
    ref += 1
    element.setAttribute('data-prime-ref', String(ref))
    const label = (element.getAttribute('aria-label') || element.innerText || element.value || element.getAttribute('placeholder') || element.getAttribute('title') || element.getAttribute('alt') || '').replace(/\\s+/g, ' ').trim().slice(0, 80)
    const tag = element.tagName.toLowerCase()
    const kind = element.getAttribute('role') || (tag === 'input' ? 'input:' + (element.type || 'text') : tag)
    const href = tag === 'a' ? ' -> ' + element.getAttribute('href') : ''
    elements.push('[' + ref + '] ' + kind + ' "' + label + '"' + href)
  }
  const text = (document.body?.innerText || '').replace(/\\n{3,}/g, '\\n\\n').slice(0, 12000)
  return { title: document.title, url: location.href, text, elements }
})()`

function text(value: string): ToolContent[] {
  return [{ type: 'text', text: value }]
}

async function snapshot(target: BrowserTarget) {
  const page = await target.contents.executeJavaScript(snapshotScript, true)
  return text(
    [
      `Tab ID: ${target.id}`,
      `URL: ${page.url}`,
      `Title: ${page.title}`,
      '',
      'Interactive elements (use ref with browser_click / browser_type):',
      ...(page.elements.length ? page.elements : ['(none)']),
      '',
      'Page text:',
      page.text || '(empty)'
    ].join('\n')
  )
}

function refSelector(ref: unknown) {
  const value = Number(ref)
  if (!Number.isInteger(value) || value < 1)
    throw new Error('ref must be a positive integer from browser_snapshot')
  return `[data-prime-ref="${value}"]`
}

async function focusRef(target: BrowserTarget, ref: unknown) {
  const selector = refSelector(ref)
  const found = await target.contents.executeJavaScript(
    `(() => { const element = document.querySelector(${JSON.stringify(selector)}); if (!element) return false; element.scrollIntoView({ block: 'center' }); element.focus(); if (element.select) element.select(); return true })()`,
    true
  )
  if (!found) throw new Error(`Element ${String(ref)} not found. Call browser_snapshot again.`)
}

function pressKey(target: BrowserTarget, key: string) {
  const keyCode = key.length === 1 ? key : key
  target.contents.sendInputEvent({ type: 'keyDown', keyCode })
  if (key.length === 1 || key === 'Enter')
    target.contents.sendInputEvent({ type: 'char', keyCode: key === 'Enter' ? '\r' : key })
  target.contents.sendInputEvent({ type: 'keyUp', keyCode })
}

const settle = () => new Promise((resolve) => setTimeout(resolve, 350))

const panelWidth = {
  type: 'number',
  description:
    'Browser panel width in pixels, 320-2560. Use about 1280 for desktop, 768 for tablet, 390 for phone.'
}

function requestedWidth(value: unknown) {
  const width = Number(value)
  return Number.isFinite(width) && width >= 320 ? Math.min(2560, width) : undefined
}

export const browserTools: Tool[] = [
  ...browserTabTools,
  {
    name: 'browser_navigate',
    description:
      'Open a URL or local file in the Prime Desktop built-in browser shown to the user. Prefer this browser over any other browser tools. The browser panel opens wide by default; pass width to choose its width.',
    inputSchema: {
      type: 'object',
      properties: {
        url: { type: 'string', description: 'URL, local file path or search text' },
        tabId: tabIdProperty,
        width: panelWidth
      },
      required: ['url']
    },
    run: async (panel, args) => {
      const target = browserTarget(panel, args)
      await panel.navigate(String(args.url || ''), true, requestedWidth(args.width), target.id)
      return snapshot(target)
    }
  },
  {
    name: 'browser_resize',
    description:
      'Change the width of the built-in browser panel to see the page as a desktop, tablet or phone layout. The window size limits the maximum width.',
    inputSchema: {
      type: 'object',
      properties: { width: panelWidth, tabId: tabIdProperty },
      required: ['width']
    },
    run: async (panel, args) => {
      const width = requestedWidth(args.width)
      if (!width) throw new Error('width must be a number of pixels between 320 and 2560')
      const target = browserTarget(panel, args)
      panel.reveal(true, width)
      await settle()
      const actual = await target.contents.executeJavaScript('window.innerWidth', true)
      return text(`Requested ${width}px. Page viewport is now ${actual}px wide.`)
    }
  },
  {
    name: 'browser_snapshot',
    description:
      'Read the current page of the built-in browser: URL, title, numbered interactive elements and visible text.',
    inputSchema: { type: 'object', properties: { tabId: tabIdProperty } },
    run: (panel, args) => snapshot(browserTarget(panel, args))
  },
  {
    name: 'browser_click',
    description: 'Click an element by its ref number from browser_snapshot.',
    inputSchema: {
      type: 'object',
      properties: { ref: { type: 'number' }, tabId: tabIdProperty },
      required: ['ref']
    },
    run: async (panel, args) => {
      const selector = refSelector(args.ref)
      const target = browserTarget(panel, args)
      const clicked = await target.contents.executeJavaScript(
        `(() => { const element = document.querySelector(${JSON.stringify(selector)}); if (!element) return false; element.scrollIntoView({ block: 'center' }); element.click(); return true })()`,
        true
      )
      if (!clicked)
        throw new Error(`Element ${String(args.ref)} not found. Call browser_snapshot again.`)
      await settle()
      return snapshot(target)
    }
  },
  {
    name: 'browser_type',
    description:
      'Type text into an input, textarea or editable element by ref from browser_snapshot. Existing text is replaced.',
    inputSchema: {
      type: 'object',
      properties: {
        ref: { type: 'number' },
        tabId: tabIdProperty,
        text: { type: 'string' },
        submit: { type: 'boolean', description: 'Press Enter after typing' }
      },
      required: ['ref', 'text']
    },
    run: async (panel, args) => {
      const target = browserTarget(panel, args)
      await focusRef(target, args.ref)
      target.contents.insertText(String(args.text ?? ''))
      if (args.submit) pressKey(target, 'Enter')
      await settle()
      return snapshot(target)
    }
  },
  {
    name: 'browser_press_key',
    description:
      'Press a keyboard key in the built-in browser, such as Enter, Tab, Escape or ArrowDown.',
    inputSchema: {
      type: 'object',
      properties: { key: { type: 'string' }, tabId: tabIdProperty },
      required: ['key']
    },
    run: async (panel, args) => {
      const target = browserTarget(panel, args)
      pressKey(target, String(args.key || ''))
      await settle()
      return snapshot(target)
    }
  },
  {
    name: 'browser_scroll',
    description: 'Scroll the current page up or down.',
    inputSchema: {
      type: 'object',
      properties: { direction: { type: 'string', enum: ['up', 'down'] }, tabId: tabIdProperty },
      required: ['direction']
    },
    run: async (panel, args) => {
      const target = browserTarget(panel, args)
      await target.contents.executeJavaScript(
        `window.scrollBy({ top: ${args.direction === 'up' ? -1 : 1} * window.innerHeight * 0.8 })`,
        true
      )
      await settle()
      return snapshot(target)
    }
  },
  {
    name: 'browser_back',
    description: 'Go back in the built-in browser history.',
    inputSchema: { type: 'object', properties: { tabId: tabIdProperty } },
    run: async (panel, args) => {
      const target = browserTarget(panel, args)
      panel.back(target.id)
      await settle()
      return snapshot(target)
    }
  },
  {
    name: 'browser_screenshot',
    description: 'Take a screenshot of the visible part of the current page.',
    inputSchema: { type: 'object', properties: { tabId: tabIdProperty } },
    run: async (panel, args) => {
      const target = browserTarget(panel, args)
      const image = await panel.captureTab(target.id)
      return [{ type: 'image', data: image.toPNG().toString('base64'), mimeType: 'image/png' }]
    }
  },
  {
    name: 'browser_evaluate',
    description:
      'Run a JavaScript expression in the current page and return its JSON result. Use for reading values the snapshot does not show.',
    inputSchema: {
      type: 'object',
      properties: { expression: { type: 'string' }, tabId: tabIdProperty },
      required: ['expression']
    },
    run: async (panel, args) => {
      const target = browserTarget(panel, args)
      const result = await target.contents.executeJavaScript(
        `(async () => { const value = await (${String(args.expression)}); return JSON.stringify(value === undefined ? null : value) })()`,
        true
      )
      return text(String(result).slice(0, 20000))
    }
  }
]

import type { WebContents } from 'electron'

export type PickedElement = {
  selector: string
  tag: string
  text: string
  html: string
  url: string
  rect: { x: number; y: number; width: number; height: number }
  styles: Record<string, string>
}

const pickScript = `new Promise((resolve) => {
  const previous = window.__primePicker
  if (previous) previous.cancel()
  const box = document.createElement('div')
  const label = document.createElement('div')
  for (const node of [box, label]) node.setAttribute('data-prime-picker', '')
  Object.assign(box.style, { position: 'fixed', zIndex: 2147483647, pointerEvents: 'none', border: '2px solid #ff7a45', background: 'rgba(255,122,69,.12)', borderRadius: '3px', transition: 'all 60ms ease', display: 'none' })
  Object.assign(label.style, { position: 'fixed', zIndex: 2147483647, pointerEvents: 'none', background: '#ff7a45', color: '#fff', font: '600 11px system-ui, sans-serif', padding: '2px 6px', borderRadius: '3px', display: 'none' })
  document.documentElement.append(box, label)
  let current = null
  const selector = (element) => {
    if (element.id) return '#' + CSS.escape(element.id)
    const parts = []
    let node = element
    while (node && node.nodeType === 1 && node !== document.documentElement && parts.length < 5) {
      let part = node.tagName.toLowerCase()
      const classes = [...node.classList].filter((name) => !/^(css|sc|jsx)-/.test(name)).slice(0, 2)
      if (classes.length) part += '.' + classes.map((name) => CSS.escape(name)).join('.')
      const parent = node.parentElement
      if (parent) {
        const same = [...parent.children].filter((child) => child.tagName === node.tagName)
        if (same.length > 1) part += ':nth-of-type(' + (same.indexOf(node) + 1) + ')'
      }
      parts.unshift(part)
      if (node.id) { parts[0] = '#' + CSS.escape(node.id); break }
      node = parent
    }
    return parts.join(' > ')
  }
  const move = (event) => {
    const element = document.elementFromPoint(event.clientX, event.clientY)
    if (!element || element.hasAttribute('data-prime-picker')) return
    current = element
    const rect = element.getBoundingClientRect()
    Object.assign(box.style, { display: 'block', left: rect.left + 'px', top: rect.top + 'px', width: rect.width + 'px', height: rect.height + 'px' })
    label.textContent = element.tagName.toLowerCase() + ' ' + Math.round(rect.width) + '×' + Math.round(rect.height)
    Object.assign(label.style, { display: 'block', left: rect.left + 'px', top: Math.max(0, rect.top - 20) + 'px' })
  }
  const finish = (value) => {
    document.removeEventListener('mousemove', move, true)
    document.removeEventListener('click', click, true)
    document.removeEventListener('keydown', key, true)
    box.remove()
    label.remove()
    delete window.__primePicker
    resolve(value)
  }
  const click = (event) => {
    event.preventDefault()
    event.stopPropagation()
    const element = current || document.elementFromPoint(event.clientX, event.clientY)
    if (!element) return finish(null)
    const rect = element.getBoundingClientRect()
    const computed = getComputedStyle(element)
    const styles = {}
    for (const name of ['display', 'position', 'width', 'height', 'margin', 'padding', 'color', 'background-color', 'font-family', 'font-size', 'font-weight', 'line-height', 'border', 'border-radius', 'box-shadow', 'gap', 'flex-direction', 'justify-content', 'align-items'])
      styles[name] = computed.getPropertyValue(name)
    box.remove()
    label.remove()
    finish({
      selector: selector(element),
      tag: element.tagName.toLowerCase(),
      text: (element.innerText || element.textContent || '').trim().slice(0, 500),
      html: element.outerHTML.slice(0, 3000),
      url: location.href,
      rect: { x: rect.left, y: rect.top, width: rect.width, height: rect.height },
      styles
    })
  }
  const key = (event) => {
    if (event.key !== 'Escape') return
    event.preventDefault()
    finish(null)
  }
  document.addEventListener('mousemove', move, true)
  document.addEventListener('click', click, true)
  document.addEventListener('keydown', key, true)
  window.__primePicker = { cancel: () => finish(null) }
})`

export async function pickElement(contents: WebContents): Promise<{ element: PickedElement; image?: Buffer } | null> {
  contents.focus()
  let stop = () => {}
  const navigated = new Promise<null>((resolve) => {
    const done = () => resolve(null)
    contents.once('did-start-navigation', done)
    contents.once('destroyed', done)
    stop = () => {
      contents.removeListener('did-start-navigation', done)
      contents.removeListener('destroyed', done)
    }
  })
  const element = (await Promise.race([
    contents.executeJavaScript(pickScript, true) as Promise<PickedElement | null>,
    navigated
  ]).finally(stop)) as PickedElement | null
  if (!element) return null
  const zoom = contents.getZoomFactor()
  const rect = {
    x: Math.max(0, Math.floor(element.rect.x * zoom)),
    y: Math.max(0, Math.floor(element.rect.y * zoom)),
    width: Math.max(1, Math.ceil(element.rect.width * zoom)),
    height: Math.max(1, Math.ceil(element.rect.height * zoom))
  }
  try {
    await new Promise((resolve) => setTimeout(resolve, 50))
    const image = await contents.capturePage(rect)
    return { element, image: image.isEmpty() ? undefined : image.toPNG() }
  } catch {
    return { element }
  }
}

export function cancelPick(contents: WebContents) {
  return contents
    .executeJavaScript('window.__primePicker && window.__primePicker.cancel()', true)
    .catch(() => {})
}

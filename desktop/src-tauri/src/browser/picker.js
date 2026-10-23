new Promise((resolve) => {
  const previous = window.__primePicker
  if (previous) previous.cancel()
  const accent = '#ff7a45'
  const overlay = (border, background) => {
    const node = document.createElement('div')
    node.setAttribute('data-prime-picker', '')
    Object.assign(node.style, { position: 'fixed', zIndex: 2147483647, pointerEvents: 'none', border, background, borderRadius: '3px', display: 'none' })
    document.documentElement.append(node)
    return node
  }
  const box = overlay('2px solid ' + accent, 'rgba(255,122,69,.12)')
  box.style.transition = 'all 60ms ease'
  const label = overlay('none', accent)
  Object.assign(label.style, { color: '#fff', font: '600 11px system-ui, sans-serif', padding: '2px 6px' })
  let current = null
  const selected = []
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
  const place = (node, rect) =>
    Object.assign(node.style, { display: 'block', left: rect.left + 'px', top: rect.top + 'px', width: rect.width + 'px', height: rect.height + 'px' })
  const placeSelected = () => selected.forEach((item) => place(item.marker, item.element.getBoundingClientRect()))
  const describe = (element) => {
    const rect = element.getBoundingClientRect()
    const computed = getComputedStyle(element)
    const styles = {}
    for (const name of ['display', 'position', 'width', 'height', 'margin', 'padding', 'color', 'background-color', 'font-family', 'font-size', 'font-weight', 'line-height', 'border', 'border-radius', 'box-shadow', 'gap', 'flex-direction', 'justify-content', 'align-items'])
      styles[name] = computed.getPropertyValue(name)
    return {
      selector: selector(element),
      tag: element.tagName.toLowerCase(),
      text: (element.innerText || element.textContent || '').trim().slice(0, 500),
      html: element.outerHTML.slice(0, 3000),
      url: location.href,
      rect: { x: rect.left, y: rect.top, width: rect.width, height: rect.height },
      styles
    }
  }
  const move = (event) => {
    const element = document.elementFromPoint(event.clientX, event.clientY)
    if (!element || element.hasAttribute('data-prime-picker')) return
    current = element
    const rect = element.getBoundingClientRect()
    place(box, rect)
    const count = selected.length ? ' · ' + selected.length : ''
    label.textContent = element.tagName.toLowerCase() + ' ' + Math.round(rect.width) + '×' + Math.round(rect.height) + count
    Object.assign(label.style, { display: 'block', width: 'auto', height: 'auto', left: rect.left + 'px', top: Math.max(0, rect.top - 20) + 'px' })
  }
  const finish = (value) => {
    document.removeEventListener('mousemove', move, true)
    document.removeEventListener('click', click, true)
    document.removeEventListener('keydown', key, true)
    document.removeEventListener('keyup', release, true)
    window.removeEventListener('scroll', placeSelected, true)
    window.removeEventListener('resize', placeSelected)
    box.remove()
    label.remove()
    selected.forEach((item) => item.marker.remove())
    delete window.__primePicker
    resolve(value)
  }
  const done = () => finish(selected.length ? selected.map((item) => describe(item.element)) : null)
  const toggle = (element) => {
    const index = selected.findIndex((item) => item.element === element)
    if (index >= 0) {
      selected.splice(index, 1)[0].marker.remove()
      return
    }
    if (selected.length >= 10) return
    const marker = overlay('2px dashed ' + accent, 'rgba(255,122,69,.18)')
    selected.push({ element, marker })
    place(marker, element.getBoundingClientRect())
  }
  const click = (event) => {
    event.preventDefault()
    event.stopPropagation()
    const element = current || document.elementFromPoint(event.clientX, event.clientY)
    if (!element) return done()
    if (event.ctrlKey || event.metaKey) return toggle(element)
    selected.splice(0).forEach((item) => item.marker.remove())
    toggle(element)
    done()
  }
  const release = (event) => {
    if ((event.key === 'Control' || event.key === 'Meta') && selected.length) done()
  }
  const key = (event) => {
    if (event.key === 'Enter' && selected.length) {
      event.preventDefault()
      done()
    }
    if (event.key !== 'Escape') return
    event.preventDefault()
    finish(null)
  }
  document.addEventListener('mousemove', move, true)
  document.addEventListener('click', click, true)
  document.addEventListener('keydown', key, true)
  document.addEventListener('keyup', release, true)
  window.addEventListener('scroll', placeSelected, true)
  window.addEventListener('resize', placeSelected)
  window.__primePicker = { cancel: () => finish(null), finish: done }
})
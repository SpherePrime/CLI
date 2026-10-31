(() => {
  const MAX = 1500
  let count = 0
  document.querySelectorAll('[data-prime-ref]').forEach(element => element.removeAttribute('data-prime-ref'))
  const label = element =>
    (
      element.getAttribute('aria-label') ||
      element.innerText ||
      element.value ||
      element.getAttribute('placeholder') ||
      element.getAttribute('title') ||
      element.getAttribute('alt') ||
      ''
    )
      .replace(/\s+/g, ' ')
      .trim()
      .slice(0, 120)
  const role = element => {
    const explicit = element.getAttribute('role')
    if (explicit) return explicit.toLowerCase()
    const tag = element.tagName.toLowerCase()
    if (tag === 'a') return 'link'
    if (tag === 'button') return 'button'
    if (tag === 'select') return 'combobox'
    if (tag === 'textarea') return 'edit'
    if (tag === 'summary') return 'button'
    if (tag === 'input') {
      const type = (element.type || 'text').toLowerCase()
      if (type === 'checkbox') return 'checkbox'
      if (type === 'radio') return 'radio'
      if (type === 'submit' || type === 'button' || type === 'reset') return 'button'
      return 'edit'
    }
    return 'group'
  }
  const actions = element => {
    const tag = element.tagName.toLowerCase()
    const list = []
    if (tag === 'button' || tag === 'a' || tag === 'summary' || element.getAttribute('role') === 'button' || element.onclick) list.push('invoke')
    if (tag === 'input' || tag === 'textarea' || element.isContentEditable) list.push('setValue')
    if (tag === 'input' && (element.type === 'checkbox' || element.type === 'radio')) list.push('toggle')
    if (tag === 'select') list.push('select')
    return list
  }
  const walk = (element, depth) => {
    if (count >= MAX) return null
    const rect = element.getBoundingClientRect()
    const style = getComputedStyle(element)
    const visible = rect.width > 0 && rect.height > 0 && style.visibility !== 'hidden' && style.display !== 'none'
    const name = label(element)
    const nodeActions = actions(element)
    const interactive = nodeActions.length > 0
    let reference = null
    if (interactive) {
      count += 1
      reference = 'n' + count
      element.setAttribute('data-prime-ref', reference)
    }
    const states = []
    if (visible && !element.disabled) states.push('enabled')
    if (element.disabled) states.push('disabled')
    if (document.activeElement === element) states.push('focused')
    if (element.checked) states.push('checked')
    const children = []
    for (const child of element.children) {
      const built = walk(child, depth + 1)
      if (built) children.push(built)
    }
    if (!interactive && !name && children.length === 0) return null
    return {
      ref: reference,
      role: role(element),
      name,
      value: typeof element.value === 'string' && element.value !== '' ? element.value : null,
      bounds: {
        x: Math.round(rect.x),
        y: Math.round(rect.y),
        width: Math.round(rect.width),
        height: Math.round(rect.height)
      },
      states,
      actions: nodeActions,
      automationId: element.id || null,
      className: typeof element.className === 'string' && element.className ? element.className : null,
      path: role(element) + '[' + name + ']',
      children
    }
  }
  const root = walk(document.body, 0)
  return {
    title: document.title,
    url: location.href,
    truncated: count >= MAX,
    nodes: root ? [root] : []
  }
})()

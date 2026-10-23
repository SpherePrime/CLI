(() => {
  const selector = 'a[href],button,input,select,textarea,summary,[role=button],[role=link],[role=checkbox],[role=tab],[role=menuitem],[contenteditable=true],[onclick]'
  document.querySelectorAll('[data-prime-ref]').forEach(element => element.removeAttribute('data-prime-ref'))
  const elements = []
  for (const element of document.querySelectorAll(selector)) {
    const rect = element.getBoundingClientRect()
    const style = getComputedStyle(element)
    if (!rect.width || !rect.height || style.visibility === 'hidden' || style.display === 'none' || elements.length >= 300) continue
    const ref = elements.length + 1
    element.setAttribute('data-prime-ref', String(ref))
    const label = (element.getAttribute('aria-label') || element.innerText || element.value || element.getAttribute('placeholder') || element.getAttribute('title') || element.getAttribute('alt') || '').replace(/\s+/g, ' ').trim().slice(0,80)
    const tag = element.tagName.toLowerCase()
    const kind = element.getAttribute('role') || (tag === 'input' ? 'input:' + (element.type || 'text') : tag)
    elements.push('[' + ref + '] ' + kind + ' "' + label + '"' + (tag === 'a' ? ' -> ' + element.getAttribute('href') : ''))
  }
  return {title: document.title, url: location.href, text: (document.body?.innerText || '').replace(/\n{3,}/g, '\n\n').slice(0,12000), elements}
})()

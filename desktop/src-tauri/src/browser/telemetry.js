(() => {
  const entries = []
  const record = (level, message, source, line) => {
    entries.push({level, message: String(message).slice(0, 4000), source, line, url: location.href, time: Date.now()})
    if (entries.length > 200) entries.shift()
  }
  for (const [method, level] of [['error', 'error'], ['warn', 'warning']]) {
    const original = console[method].bind(console)
    console[method] = (...values) => {
      record(level, values.map(value => { try { return typeof value === 'string' ? value : JSON.stringify(value) } catch { return String(value) } }).join(' '))
      original(...values)
    }
  }
  window.addEventListener('error', event => record('error', event.message || 'Resource load failed', event.filename, event.lineno), true)
  window.addEventListener('unhandledrejection', event => record('error', String(event.reason)))
  Object.defineProperty(window, '__primeBrowserRead', {value: () => ({url: location.href, title: document.title, loading: document.readyState !== 'complete', logs: entries.splice(0)})})
})()

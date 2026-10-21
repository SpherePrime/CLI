import { useEffect } from 'react'

export function useModalWindowChrome() {
  useEffect(() => {
    let dimmed = false
    const sync = () => {
      const next = Boolean(document.querySelector('dialog.modal[open]'))
      if (next === dimmed) return
      dimmed = next
      void window.prime.windowDimmed?.(next).catch(() => {})
    }
    const observer = new MutationObserver(sync)
    observer.observe(document.body, {
      subtree: true,
      childList: true,
      attributes: true,
      attributeFilter: ['open']
    })
    sync()
    return () => {
      observer.disconnect()
      if (dimmed) void window.prime.windowDimmed?.(false).catch(() => {})
    }
  }, [])
}

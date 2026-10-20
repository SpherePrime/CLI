import { useEffect, useState, type RefObject } from 'react'

export function useLayoutWidth(shell: RefObject<HTMLElement | null>, zoom: number) {
  const [width, setWidth] = useState(window.innerWidth)
  useEffect(() => {
    const element = shell.current
    if (!element) return
    const update = () => setWidth(element.clientWidth)
    const observer = new ResizeObserver(update)
    observer.observe(element)
    update()
    return () => observer.disconnect()
  }, [shell, zoom])
  return width
}

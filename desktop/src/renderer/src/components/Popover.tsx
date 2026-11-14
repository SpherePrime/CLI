import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
  type RefObject
} from 'react'
import { createPortal } from 'react-dom'

type PopoverEntry = { id: number; close: () => void }
const popoverStack: PopoverEntry[] = []
let nextPopoverId = 1

export function Popover({
  anchor,
  close,
  children,
  width = 360,
  label,
  align = 'start'
}: {
  anchor: RefObject<HTMLElement | null>
  close: () => void
  children: ReactNode
  width?: number
  label?: string
  align?: 'start' | 'end'
}) {
  const panel = useRef<HTMLDivElement>(null)
  const [placement, setPlacement] = useState<'above' | 'below'>('above')
  const [position, setPosition] = useState({ left: 0, top: 0, maxHeight: 400 })
  const popoverId = useMemo(() => nextPopoverId++, [])
  const closeRef = useRef(close)
  closeRef.current = close

  useEffect(() => {
    const nested = Boolean(anchor.current?.closest('.prime-popover'))
    if (!nested) {
      for (const entry of [...popoverStack]) {
        if (entry.id !== popoverId) entry.close()
      }
    }
    popoverStack.push({ id: popoverId, close: () => closeRef.current() })
    return () => {
      const index = popoverStack.findIndex((entry) => entry.id === popoverId)
      if (index !== -1) popoverStack.splice(index, 1)
    }
  }, [anchor, popoverId])

  useLayoutEffect(() => {
    const place = () => {
      const rect = anchor.current?.getBoundingClientRect()
      if (!rect) return
      const above = rect.top > window.innerHeight - rect.bottom
      setPlacement(above ? 'above' : 'below')
      const height = Math.min(
        panel.current?.scrollHeight || 400,
        above ? rect.top - 20 : window.innerHeight - rect.bottom - 20
      )
      const preferred = align === 'end' ? rect.right - width : rect.left
      setPosition({
        left: Math.max(12, Math.min(preferred, window.innerWidth - width - 12)),
        top: above ? Math.max(12, rect.top - height - 8) : rect.bottom + 8,
        maxHeight: above ? rect.top - 20 : window.innerHeight - rect.bottom - 20
      })
    }
    place()
    const observer = new ResizeObserver(place)
    if (panel.current) observer.observe(panel.current)
    window.addEventListener('resize', place)
    return () => {
      observer.disconnect()
      window.removeEventListener('resize', place)
    }
  }, [anchor, width, align])

  useEffect(() => {
    const pointer = (event: PointerEvent) => {
      if ((event.target as Element).closest?.('.prime-popover')) return
      if (
        !panel.current?.contains(event.target as Node) &&
        !anchor.current?.contains(event.target as Node)
      )
        closeRef.current()
    }
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        const top = popoverStack[popoverStack.length - 1]
        if (top && top.id !== popoverId) return
        event.preventDefault()
        event.stopPropagation()
        event.stopImmediatePropagation()
        closeRef.current()
        anchor.current?.focus()
      }
    }
    document.addEventListener('pointerdown', pointer)
    document.addEventListener('keydown', key, true)
    return () => {
      document.removeEventListener('pointerdown', pointer)
      document.removeEventListener('keydown', key, true)
    }
  }, [anchor, popoverId])

  return createPortal(
    <div
      ref={panel}
      className="prime-popover"
      data-placement={placement}
      aria-label={label}
      style={{ ...position, width: Math.min(width, window.innerWidth - 24) }}
    >
      {children}
    </div>,
    anchor.current?.closest('dialog') || document.body
  )
}

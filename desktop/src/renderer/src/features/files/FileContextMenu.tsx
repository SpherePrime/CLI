import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

export type MenuItem =
  { separator: true } | { label: string; icon?: ReactNode; danger?: boolean; action: () => void }

export function FileContextMenu({
  x,
  y,
  items,
  close
}: {
  x: number
  y: number
  items: MenuItem[]
  close: () => void
}) {
  const menu = useRef<HTMLDivElement>(null)
  const [position, setPosition] = useState({ left: x, top: y })
  useLayoutEffect(() => {
    const rect = menu.current?.getBoundingClientRect()
    if (!rect) return
    setPosition({
      left: Math.max(8, Math.min(x, window.innerWidth - rect.width - 8)),
      top: Math.max(8, Math.min(y, window.innerHeight - rect.height - 8))
    })
    menu.current?.querySelector<HTMLElement>('[role=menuitem]')?.focus()
  }, [x, y])
  useEffect(() => {
    const pointer = (event: PointerEvent) => {
      if (!menu.current?.contains(event.target as Node)) close()
    }
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        close()
      }
    }
    document.addEventListener('pointerdown', pointer, true)
    document.addEventListener('keydown', key, true)
    window.addEventListener('blur', close)
    window.addEventListener('resize', close)
    return () => {
      document.removeEventListener('pointerdown', pointer, true)
      document.removeEventListener('keydown', key, true)
      window.removeEventListener('blur', close)
      window.removeEventListener('resize', close)
    }
  }, [close])
  return createPortal(
    <div
      ref={menu}
      className="file-context-menu"
      role="menu"
      style={position}
      onContextMenu={(event) => event.preventDefault()}
      onKeyDown={(event) => {
        if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return
        event.preventDefault()
        const buttons = Array.from(
          event.currentTarget.querySelectorAll<HTMLButtonElement>('[role=menuitem]')
        )
        const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
        const step = event.key === 'ArrowDown' ? 1 : -1
        buttons[(index + step + buttons.length) % buttons.length]?.focus()
      }}
    >
      {items.map((item, index) =>
        'separator' in item ? (
          <hr key={index} />
        ) : (
          <button
            key={index}
            role="menuitem"
            className={item.danger ? 'danger' : ''}
            onClick={() => {
              close()
              item.action()
            }}
          >
            {item.icon}
            <span>{item.label}</span>
          </button>
        )
      )}
    </div>,
    document.body
  )
}

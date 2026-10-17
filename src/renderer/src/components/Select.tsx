import { useRef, useState } from 'react'
import { Check, ChevronDown } from 'lucide-react'
import { Popover } from './Popover'

export function Select({
  value,
  onChange,
  options,
  label,
  disabled = false
}: {
  value: string
  onChange: (value: string) => void
  options: { value: string; label: string }[]
  label: string
  disabled?: boolean
}) {
  const anchor = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  function choose(next: string) {
    onChange(next)
    setOpen(false)
    anchor.current?.focus()
  }
  return (
    <>
      <button
        ref={anchor}
        type="button"
        className="prime-select"
        role="combobox"
        aria-label={label}
        aria-expanded={open}
        aria-haspopup="listbox"
        disabled={disabled}
        onClick={() => setOpen(!open)}
        onKeyDown={(event) => {
          if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
            event.preventDefault()
            setOpen(true)
          }
        }}
      >
        <span>{options.find((item) => item.value === value)?.label || value}</span>
        <ChevronDown size={14} />
      </button>
      {open && (
        <Popover
          anchor={anchor}
          close={() => setOpen(false)}
          width={Math.max(230, anchor.current?.getBoundingClientRect().width || 0)}
        >
          <div
            className="select-options"
            role="listbox"
            aria-label={label}
            onKeyDown={(event) => {
              const buttons = Array.from(
                event.currentTarget.querySelectorAll<HTMLButtonElement>('[role=option]')
              )
              const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
              if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
                event.preventDefault()
                buttons[
                  event.key === 'Home'
                    ? 0
                    : event.key === 'End'
                      ? buttons.length - 1
                      : (index + (event.key === 'ArrowUp' ? -1 : 1) + buttons.length) %
                        buttons.length
                ]?.focus()
              }
            }}
          >
            {options.map((item) => (
              <button
                key={item.value}
                role="option"
                aria-selected={item.value === value}
                autoFocus={item.value === value}
                onClick={() => choose(item.value)}
              >
                <span>{item.label}</span>
                {item.value === value && <Check size={14} />}
              </button>
            ))}
          </div>
        </Popover>
      )}
    </>
  )
}

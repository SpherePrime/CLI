import { useI18n } from '../i18n'
import { useEffect, useId, useRef, type RefObject } from 'react'
import { X } from 'lucide-react'
export function Modal({
  title,
  close,
  children,
  wide = false,
  className = '',
  initialFocus
}: {
  title: string
  close: () => void
  children: React.ReactNode
  wide?: boolean
  className?: string
  initialFocus?: RefObject<HTMLElement | null>
}) {
  const { t } = useI18n()
  const ref = useRef<HTMLDialogElement>(null)
  const titleId = useId()
  useEffect(() => {
    ref.current?.showModal()
    initialFocus?.current?.focus()
    return () => ref.current?.close()
  }, [])
  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      className={`modal ${wide ? 'wide' : ''} ${className}`}
      onCancel={(event) => {
        event.preventDefault()
        close()
      }}
      onClick={(event) => {
        if (event.target === ref.current) close()
      }}
    >
      <header>
        <h2 id={titleId}>{title}</h2>
        <button className="icon" aria-label={t('Закрыть')} onClick={close}>
          <X size={18} />
        </button>
      </header>
      <div className="modal-content">{children}</div>
    </dialog>
  )
}

import { useI18n } from '../i18n'
import { useEffect, useId, useRef, type ReactNode, type RefObject } from 'react'
import { X } from 'lucide-react'
export function Modal({
  title,
  close,
  children,
  wide = false,
  bare = false,
  lead,
  className = '',
  initialFocus
}: {
  title: string
  close: () => void
  children: ReactNode
  wide?: boolean
  bare?: boolean
  lead?: ReactNode
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
      className={`modal ${wide ? 'wide' : ''} ${bare ? 'bare' : ''} ${className}`}
      onCancel={(event) => {
        event.preventDefault()
        close()
      }}
      onClick={(event) => {
        if (event.target === ref.current) close()
      }}
    >
      {bare ? (
        <>
          <button className="icon modal-close" aria-label={t('Закрыть')} onClick={close}>
            <X size={16} />
          </button>
          {lead}
          <h2 id={titleId} className="modal-title">
            {title}
          </h2>
        </>
      ) : (
        <header>
          <h2 id={titleId}>{title}</h2>
          <button className="icon" aria-label={t('Закрыть')} onClick={close}>
            <X size={18} />
          </button>
        </header>
      )}
      <div className="modal-content">{children}</div>
    </dialog>
  )
}

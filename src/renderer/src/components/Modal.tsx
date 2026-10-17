import { useI18n } from '../i18n'
import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
export function Modal({
  title,
  close,
  children,
  wide = false,
  className = ''
}: {
  title: string
  close: () => void
  children: React.ReactNode
  wide?: boolean
  className?: string
}) {
  const { t } = useI18n()
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    ref.current?.showModal()
    return () => ref.current?.close()
  }, [])
  return (
    <dialog
      ref={ref}
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
        <h2>{title}</h2>
        <button className="icon" aria-label={t('Закрыть')} onClick={close}>
          <X size={18} />
        </button>
      </header>
      <div className="modal-content">{children}</div>
    </dialog>
  )
}

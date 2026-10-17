import { useRef, useState } from 'react'
import { Check, ChevronDown, ShieldCheck, ShieldOff } from 'lucide-react'
import { Popover } from '../../components/Popover'
import { useI18n } from '../../i18n'
export function PermissionMenu({
  skip,
  disabled,
  onChange
}: {
  skip: boolean
  disabled: boolean
  onChange: (skip: boolean) => Promise<void>
}) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const anchor = useRef<HTMLButtonElement>(null)
  return (
    <>
      <button
        ref={anchor}
        className={`permission-trigger ${skip ? 'unrestricted' : ''}`}
        aria-label={t('Режим доступа')}
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={disabled || saving}
        onClick={() => setOpen(!open)}
      >
        {skip ? <ShieldOff size={15} /> : <ShieldCheck size={15} />}
        <span>{skip ? t('Полный доступ') : t('С подтверждением')}</span>
        <ChevronDown size={11} />
      </button>
      {open && (
        <Popover anchor={anchor} close={() => setOpen(false)} width={330}>
          <div className="permission-menu" role="menu" aria-label={t('Доступ инструментов')}>
            <strong>{t('Доступ инструментов')}</strong>
            {[
              {
                skip: false,
                title: t('С подтверждением'),
                description: t('Запрашивать разрешение на защищённые действия.')
              },
              {
                skip: true,
                title: t('Полный доступ'),
                description: t('Выполнять действия без дополнительных подтверждений.')
              }
            ].map((item) => (
              <button
                key={String(item.skip)}
                role="menuitemradio"
                aria-checked={skip === item.skip}
                onClick={async () => {
                  setSaving(true)
                  try {
                    await onChange(item.skip)
                    setOpen(false)
                  } finally {
                    setSaving(false)
                  }
                }}
              >
                <span className="permission-choice-icon">
                  {item.skip ? <ShieldOff size={18} /> : <ShieldCheck size={18} />}
                </span>
                <span>
                  <b>{item.title}</b>
                  <small>{item.description}</small>
                </span>
                {skip === item.skip && <Check size={15} />}
              </button>
            ))}
          </div>
        </Popover>
      )}
    </>
  )
}

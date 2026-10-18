import { useEffect, useRef, useState } from 'react'
import { MoreHorizontal, ListRestart, FileText } from 'lucide-react'
import { Popover } from '../../components/Popover'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function ChatActions({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [templates, setTemplates] = useState<any[]>([])
  const anchor = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    let current = true
    if (open && w.project)
      void Promise.all([
        w.call('commands').catch(() => []),
        w.call('mcpPrompts').catch(() => [])
      ]).then((result) => {
        if (current)
          setTemplates(
            result.flat().filter((item) => item && (item.Name || item.name || item.title))
          )
      })
    return () => {
      current = false
    }
  }, [open, w.project?.id])
  return (
    <>
      <button
        ref={anchor}
        className="chat-actions-trigger"
        aria-label={t('Действия чата')}
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <MoreHorizontal size={18} />
        <span>{t('Действия')}</span>
      </button>
      {open && (
        <Popover anchor={anchor} close={() => setOpen(false)} width={290}>
          <div className="action-list">
            <button
              disabled={!w.session || w.busy}
              onClick={() => {
                setOpen(false)
                void w
                  .call('summarize', { sessionId: w.session?.id })
                  .then(w.refresh)
                  .catch(w.errorHandler)
              }}
            >
              <ListRestart size={16} />
              {t('Сжать контекст')}
            </button>
            {templates.length > 0 && <div className="action-caption">{t('Шаблоны запросов')}</div>}
            {templates.map((item, index) => (
              <button
                key={index}
                onClick={() => {
                  setOpen(false)
                  window.dispatchEvent(new CustomEvent('prime:command', { detail: item }))
                }}
              >
                <FileText size={15} />
                <span>{item.Name || item.name || item.title}</span>
              </button>
            ))}
          </div>
        </Popover>
      )}
    </>
  )
}

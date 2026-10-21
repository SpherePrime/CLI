import { shortcutLabel } from '../../../../shared/keybindings'
import { useShortcuts } from '../shortcuts/shortcutStore'
import { useEffect, useRef, useState } from 'react'
import { MoreHorizontal, ListRestart, FileText, Download, Copy, Check, Focus } from 'lucide-react'
import { Popover } from '../../components/Popover'
import type { Workspace } from '../../app/useWorkspace'
import { downloadConversation } from './exportConversation'
import { exportConversation } from '../../../../shared/workspaceTools'
import { useI18n } from '../../i18n'

export function ChatActions({
  workspace: w,
  focusMode,
  toggleFocus
}: {
  workspace: Workspace
  focusMode: boolean
  toggleFocus: () => void
}) {
  const { t } = useI18n()
  const shortcuts = useShortcuts()
  const [open, setOpen] = useState(false)
  const [copied, setCopied] = useState(false)
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
        className={`icon ${open ? 'active-control' : ''}`}
        title={t('Действия чата')}
        aria-label={t('Действия чата')}
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <MoreHorizontal size={18} />
      </button>
      {open && (
        <Popover anchor={anchor} close={() => setOpen(false)} width={290}>
          <div className="action-list">
            <button
              aria-pressed={focusMode}
              onClick={() => {
                setOpen(false)
                toggleFocus()
              }}
            >
              <Focus size={16} />
              {focusMode ? t('Выйти из фокуса') : t('Режим фокуса')}
              <kbd>{shortcutLabel(shortcuts.focus)}</kbd>
            </button>
            <button
              disabled={!w.messages.length}
              onClick={() => {
                downloadConversation(
                  w.session?.title || t('Новый чат'),
                  w.messages,
                  t
                )
                setOpen(false)
              }}
            >
              <Download size={16} />
              {t('Экспорт в Markdown')}
            </button>
            <button
              disabled={!w.messages.length}
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(
                    exportConversation(w.session?.title || t('Новый чат'), w.messages, {
                      user: 'User',
                      assistant: 'Prime',
                      exported: t('Экспорт: {value0}')
                    })
                  )
                  setCopied(true)
                  setTimeout(() => setCopied(false), 1500)
                } catch (error) {
                  w.errorHandler(error)
                }
              }}
            >
              {copied ? <Check size={16} /> : <Copy size={16} />}
              {copied ? t('Скопировано') : t('Скопировать чат')}
            </button>
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

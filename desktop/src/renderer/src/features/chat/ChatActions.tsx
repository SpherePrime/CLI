import { sessionTitle } from '../../../../shared/sessionTitle'
import { shortcutLabel } from '../../../../shared/keybindings'
import { useShortcuts } from '../shortcuts/shortcutStore'
import { useEffect, useRef, useState } from 'react'
import { MoreHorizontal, ListRestart, FileText, Download, Copy, Check, Focus } from 'lucide-react'
import { Popover } from '../../components/Popover'
import type { Workspace } from '../../app/useWorkspace'
import { downloadConversationChunks } from './exportConversation'
import { conversationExportChunks } from './pagedExport'
import { realSessionId } from '../../../../shared/pendingSession'
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
  const [exporting, setExporting] = useState(false)
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
              disabled={!w.messages.length || exporting}
              onClick={async () => {
                if (!w.project || !w.session) return
                setExporting(true)
                try {
                  const title = sessionTitle(w.session.title, t('Новый чат'))
                  const chunks = await conversationExportChunks(
                    w.project.id,
                    w.session.id,
                    title,
                    t
                  )
                  downloadConversationChunks(title, chunks)
                  setOpen(false)
                } catch (error) {
                  w.errorHandler(error)
                } finally {
                  setExporting(false)
                }
              }}
            >
              <Download size={16} />
              {t('Экспорт в Markdown')}
            </button>
            <button
              disabled={!w.messages.length || exporting}
              onClick={async () => {
                setExporting(true)
                try {
                  if (!w.project || !w.session) return
                  const chunks = await conversationExportChunks(
                    w.project.id,
                    w.session.id,
                    sessionTitle(w.session.title, t('Новый чат')),
                    t
                  )
                  await navigator.clipboard.writeText(chunks.join(''))
                  setCopied(true)
                  setTimeout(() => setCopied(false), 1500)
                } catch (error) {
                  w.errorHandler(error)
                } finally {
                  setExporting(false)
                }
              }}
            >
              {copied ? <Check size={16} /> : <Copy size={16} />}
              {copied ? t('Скопировано') : t('Скопировать чат')}
            </button>
            <button
              disabled={!realSessionId(w.session) || w.busy}
              onClick={() => {
                setOpen(false)
                void w
                  .call('summarize', { sessionId: realSessionId(w.session) })
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

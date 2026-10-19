import { useCallback, useLayoutEffect, useRef, useState } from 'react'
import { Clock3, Maximize2, Minimize2, TerminalSquare, X } from 'lucide-react'
import { TerminalPanel } from '../terminal/TerminalPanel'
import type { Workspace } from '../../app/useWorkspace'
import type { TerminalRequest } from '../../../../shared/terminal'
import { useI18n } from '../../i18n'
import '../../styles/workbench-dock.css'
import { AutomationPanel } from '../automations/AutomationPanel'
import type { AutomationRequest } from '../../../../shared/automations'
import { openWorkbenchRun } from './openRun'

export type WorkbenchView = 'terminal' | 'automations'

export function WorkbenchDock({
  workspace: w,
  expanded,
  view,
  setView,
  setExpanded,
  close
}: {
  workspace: Workspace
  expanded: boolean
  view: WorkbenchView
  setView: (view: WorkbenchView) => void
  setExpanded: (expanded: boolean) => void
  close: () => void
}) {
  const { t } = useI18n()
  const [height, setHeight] = useState(340)
  const [availableHeight, setAvailableHeight] = useState(340)
  const [headerHost, setHeaderHost] = useState<HTMLDivElement | null>(null)
  const dock = useRef<HTMLElement>(null)
  useLayoutEffect(() => {
    const element = dock.current
    const parent = element?.parentElement
    if (!element || !parent || expanded) return
    const fixed = () =>
      [...parent.children].filter(
        (item) => item !== element && !item.matches('.welcome,.conversation-panel')
      ) as HTMLElement[]
    const measure = () =>
      setAvailableHeight(
        Math.max(
          0,
          parent.clientHeight -
            fixed().reduce((sum, item) => sum + item.getBoundingClientRect().height, 0) -
            40
        )
      )
    const observer = new ResizeObserver(measure)
    observer.observe(parent)
    fixed().forEach((item) => observer.observe(item))
    const changes = new MutationObserver(() => {
      fixed().forEach((item) => observer.observe(item))
      measure()
    })
    changes.observe(parent, { childList: true })
    measure()
    return () => {
      observer.disconnect()
      changes.disconnect()
    }
  }, [expanded])
  const workspaceId = w.project?.id || ''
  const terminal = useCallback(
    (request: TerminalRequest) => window.prime.terminal(workspaceId, request),
    [workspaceId]
  )
  const automation = useCallback(
    (request: AutomationRequest) => window.prime.automation(workspaceId, request),
    [workspaceId]
  )
  return (
    <section
      ref={dock}
      className={`workbench-dock ${expanded ? 'expanded' : ''}`}
      style={expanded ? undefined : { height: Math.min(height, availableHeight) }}
      aria-label={t('Инструменты проекта')}
    >
      {!expanded && (
        <div
          className="workbench-divider"
          role="separator"
          aria-orientation="horizontal"
          aria-label={t('Высота панели инструментов')}
          tabIndex={0}
          onKeyDown={(event) => {
            if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
              event.preventDefault()
              setHeight((value) =>
                Math.max(
                  140,
                  Math.min(availableHeight, value + (event.key === 'ArrowUp' ? 20 : -20))
                )
              )
            }
          }}
          onPointerDown={(event) => {
            event.preventDefault()
            const start = event.clientY
            const initial = height
            const move = (pointer: PointerEvent) =>
              setHeight(Math.max(140, Math.min(availableHeight, initial + start - pointer.clientY)))
            const stop = () => {
              window.removeEventListener('pointermove', move)
              window.removeEventListener('pointerup', stop)
            }
            window.addEventListener('pointermove', move)
            window.addEventListener('pointerup', stop)
          }}
        />
      )}
      <header className="workbench-navigation">
        <nav className="workbench-tabs" role="tablist" aria-label={t('Инструменты проекта')}>
          <button
            role="tab"
            aria-selected={view === 'terminal'}
            onClick={() => setView('terminal')}
          >
            <TerminalSquare size={14} />
            {t('Терминал')}
          </button>
          <button
            role="tab"
            aria-selected={view === 'automations'}
            onClick={() => {
              setView('automations')
              setExpanded(true)
            }}
          >
            <Clock3 size={14} />
            {t('Автоматизации')}
          </button>
        </nav>
        <div>
          <div className="workbench-terminal-actions" ref={setHeaderHost} />
          <button
            className="icon small"
            aria-label={expanded ? t('Свернуть панель') : t('Развернуть панель')}
            title={expanded ? t('Свернуть панель') : t('Развернуть панель')}
            onClick={() => setExpanded(!expanded)}
          >
            {expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
          </button>
          <button
            className="icon small"
            aria-label={t('Скрыть инструменты')}
            title={t('Скрыть инструменты')}
            onClick={close}
          >
            <X size={15} />
          </button>
        </div>
      </header>
      <div className="workbench-content">
        {w.project && view === 'terminal' && (
          <TerminalPanel
            workspaceId={workspaceId}
            api={terminal}
            subscribe={window.prime.onEvent}
            onError={w.errorHandler}
            theme={w.preferences.theme}
            headerHost={headerHost}
          />
        )}
        {w.project && view === 'automations' && (
          <AutomationPanel
            root={w.project.path}
            api={automation}
            subscribe={window.prime.onEvent}
            onError={w.errorHandler}
            onOpenRun={(run) => {
              void openWorkbenchRun(w, run).then(close).catch(w.errorHandler)
            }}
          />
        )}
      </div>
    </section>
  )
}

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { PanelBoundary } from '../../components/PanelBoundary'
import {
  AppWindow,
  Clock3,
  Github,
  Maximize2,
  Minimize2,
  Rocket,
  TerminalSquare,
  Users,
  X
} from 'lucide-react'
import { GithubPanel } from '../github/GithubPanel'
import { WindowsPanel } from '../windows/WindowsPanel'
import { startAgentTask } from './agentTask'
import { LaunchPanel } from '../launch/LaunchPanel'
import type { LaunchRequest } from '../../../../shared/launch'
import { TeamPanel } from '../teams/TeamPanel'
import type { TeamRequest } from '../../../../shared/teams'
import { TerminalPanel } from '../terminal/TerminalPanel'
import type { Workspace } from '../../app/useWorkspace'
import type { TerminalRequest } from '../../../../shared/terminal'
import { useI18n } from '../../i18n'
import '../../styles/workbench-dock.css'
import { AutomationPanel } from '../automations/AutomationPanel'
import type { AutomationRequest } from '../../../../shared/automations'
import { openWorkbenchRun } from './openRun'
import { realSessionId } from '../../../../shared/pendingSession'
import {
  clampWorkbenchHeight,
  defaultWorkbenchHeight,
  workbenchHeightBounds
} from './workbenchHeight'

export type WorkbenchView =
  | 'terminal'
  | 'automations'
  | 'teams'
  | 'launch'
  | 'github'
  | 'windows'

const minimumConversation = 120

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
  const [height, setHeight] = useState(() =>
    clampWorkbenchHeight(w.preferences.workbenchHeight ?? defaultWorkbenchHeight(window.innerHeight))
  )
  const [availableHeight, setAvailableHeight] = useState(340)
  const [headerHost, setHeaderHost] = useState<HTMLDivElement | null>(null)
  const dock = useRef<HTMLElement>(null)
  const dragging = useRef(false)
  useEffect(() => {
    if (dragging.current || typeof w.preferences.workbenchHeight !== 'number') return
    setHeight(clampWorkbenchHeight(w.preferences.workbenchHeight))
  }, [w.preferences.workbenchHeight])
  const persistHeight = useCallback(
    (next: number) => {
      void w
        .savePreferences({ workbenchHeight: clampWorkbenchHeight(next) })
        .catch(w.errorHandler)
    },
    [w.savePreferences, w.errorHandler]
  )
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
          ((element.closest('.app-shell') as HTMLElement | null)?.clientHeight ??
            parent.clientHeight) -
            fixed().reduce((sum, item) => sum + item.getBoundingClientRect().height, 0) -
            minimumConversation
        )
      )
    let frame = 0
    const scheduleMeasure = () => {
      if (frame) return
      frame = requestAnimationFrame(() => {
        frame = 0
        measure()
      })
    }
    const observer = new ResizeObserver(scheduleMeasure)
    observer.observe(parent)
    fixed().forEach((item) => observer.observe(item))
    const changes = new MutationObserver(() => {
      fixed().forEach((item) => observer.observe(item))
      scheduleMeasure()
    })
    changes.observe(parent, { childList: true })
    measure()
    return () => {
      if (frame) cancelAnimationFrame(frame)
      observer.disconnect()
      changes.disconnect()
    }
  }, [expanded])
  const tabs: { id: WorkbenchView; label: string; icon: typeof TerminalSquare }[] = [
    { id: 'terminal', label: t('Терминал'), icon: TerminalSquare },
    { id: 'launch', label: t('Запуск'), icon: Rocket },
    { id: 'automations', label: t('Автоматизации'), icon: Clock3 },
    { id: 'teams', label: t('Команды'), icon: Users },
    { id: 'github', label: 'GitHub', icon: Github },
    { id: 'windows', label: t('Окна'), icon: AppWindow }
  ]
  const workspaceId = w.project?.id || ''
  const terminal = useCallback(
    (request: TerminalRequest) => window.prime.terminal(workspaceId, request),
    [workspaceId]
  )
  const automation = useCallback(
    (request: AutomationRequest) => window.prime.automation(workspaceId, request),
    [workspaceId]
  )
  const launch = useCallback(
    (request: LaunchRequest) => window.prime.launch(workspaceId, request),
    [workspaceId]
  )
  const team = useCallback(
    (request: TeamRequest) => window.prime.team(workspaceId, request),
    [workspaceId]
  )
  const maxAllowed = Math.max(
    workbenchHeightBounds.min,
    Math.min(workbenchHeightBounds.max, Math.round(availableHeight))
  )
  const clampedHeight = clampWorkbenchHeight(height, availableHeight)
  const renderedHeight =
    availableHeight >= workbenchHeightBounds.min
      ? clampedHeight
      : Math.max(48, Math.round(availableHeight))
  const minBound = Math.min(workbenchHeightBounds.min, renderedHeight)
  const maxBound = Math.max(renderedHeight, maxAllowed)
  return (
    <section
      ref={dock}
      className={`workbench-dock ${expanded ? 'expanded' : ''}`}
      style={expanded ? undefined : { height: renderedHeight }}
      aria-label={t('Инструменты проекта')}
    >
      {!expanded && (
        <div
          className="workbench-divider"
          role="separator"
          aria-orientation="horizontal"
          aria-label={t('Высота панели инструментов')}
          aria-valuemin={minBound}
          aria-valuemax={maxBound}
          aria-valuenow={renderedHeight}
          tabIndex={0}
          onKeyDown={(event) => {
            if (!['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
            event.preventDefault()
            const step = event.shiftKey ? 50 : 20
            const raw =
              event.key === 'Home'
                ? workbenchHeightBounds.min
                : event.key === 'End'
                  ? maxAllowed
                  : clampedHeight + (event.key === 'ArrowUp' ? step : -step)
            const next = clampWorkbenchHeight(raw, availableHeight)
            setHeight(next)
            persistHeight(next)
          }}
          onPointerDown={(event) => {
            event.preventDefault()
            const target = event.currentTarget
            const pointerId = event.pointerId
            target.setPointerCapture?.(pointerId)
            dragging.current = true
            const start = event.clientY
            const initial = clampedHeight
            let latest = initial
            const move = (pointer: PointerEvent) => {
              latest = clampWorkbenchHeight(initial + start - pointer.clientY, availableHeight)
              setHeight(latest)
            }
            const stop = () => {
              dragging.current = false
              window.removeEventListener('pointermove', move)
              window.removeEventListener('pointerup', stop)
              window.removeEventListener('pointercancel', stop)
              if (target.hasPointerCapture?.(pointerId)) target.releasePointerCapture(pointerId)
              if (latest !== initial) persistHeight(latest)
            }
            window.addEventListener('pointermove', move)
            window.addEventListener('pointerup', stop)
            window.addEventListener('pointercancel', stop)
          }}
        />
      )}
      <header className="workbench-navigation">
        <nav className="workbench-tabs" role="tablist" aria-label={t('Инструменты проекта')}>
          {tabs.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              role="tab"
              aria-selected={view === id}
              title={label}
              aria-label={label}
              onClick={() => setView(id)}
            >
              <Icon size={14} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
        <div className="workbench-navigation-actions">
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
        <PanelBoundary resetKey={view}>
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
          {w.project && view === 'launch' && (
            <LaunchPanel
              root={w.project.path}
              api={launch}
              terminal={terminal}
              subscribe={window.prime.onEvent}
              theme={w.preferences.theme}
              onError={w.errorHandler}
              onDiagnostics={(text) => {
                window.dispatchEvent(new CustomEvent('prime:draft', { detail: text }))
                setExpanded(false)
              }}
            />
          )}
          {w.project && view === 'windows' && (
            <WindowsPanel
              workspaceId={workspaceId}
              sessionId={realSessionId(w.session)}
              sessionTitle={w.session?.title}
              onError={w.errorHandler}
            />
          )}
          {w.project && view === 'github' && (
            <GithubPanel
              workspaceId={workspaceId}
              onError={w.errorHandler}
              onAgentTask={async (title, prompt) => {
                await startAgentTask(w, title, prompt)
                close()
              }}
            />
          )}
          {w.project && view === 'teams' && (
            <TeamPanel
              root={w.project.path}
              api={team}
              subscribe={window.prime.onEvent}
              onError={w.errorHandler}
              onOpenChat={(run) => {
                void openWorkbenchRun(w, run).then(close).catch(w.errorHandler)
              }}
            />
          )}
        </PanelBoundary>
      </div>
    </section>
  )
}

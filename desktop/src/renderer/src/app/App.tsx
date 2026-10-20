import { panelGeometry } from './panelGeometry'
import { useLayoutWidth } from './useLayoutWidth'
import { ProjectRail } from '../features/projects/ProjectRail'
import { projectName as projectLabel } from '../features/projects/projectName'
import { useDocuments } from '../features/files/useDocuments'
import { StartupScreen } from '../components/StartupScreen'
import { UpdateScreen } from '../features/updates/UpdateScreen'
import { Titlebar } from '../components/Titlebar'
import { I18nProvider, useI18n } from '../i18n'
import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import {
  GitBranch,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightOpen,
  Search,
  X,
  AlertCircle,
  ChevronRight,
  Command,
  Focus,
  Settings2,
  Plus,
  Download,
  SunMoon,
  Puzzle,
  TerminalSquare,
  Clock3,
  Server,
  Users
} from 'lucide-react'
import { useWorkspace, type Workspace } from './useWorkspace'
import { Sidebar } from '../features/projects/Sidebar'
import { MessageList } from '../features/chat/MessageList'
import { Composer } from '../features/chat/Composer'
import { Approvals } from '../features/approvals/Approvals'
import { Inspector } from '../features/files/Inspector'
import { SettingsDialog } from '../features/settings/SettingsDialog'
import { PluginManager } from '../features/plugins/PluginManager'
import { ChatActions } from '../features/chat/ChatActions'
import { Welcome } from '../features/projects/Welcome'
import { CommandPalette, type QuickAction } from '../features/navigation/CommandPalette'
import { downloadConversation } from '../features/chat/exportConversation'
import { WorkbenchDock, type WorkbenchView } from '../features/workbench/WorkbenchDock'
import { RemoteDialog } from '../features/remote/RemoteDialog'
import { JobsCenter } from '../features/jobs/JobsCenter'
import { ProjectLockBanner } from '../features/projects/ProjectLockBanner'
import { WindowControlBadge } from '../features/windows/WindowControlBadge'

const FileEditor = lazy(() => import('../features/files/FileEditor'))

export function App() {
  const w = useWorkspace()
  const [started, setStarted] = useState(false)
  const failed = Boolean(w.error || w.info.error || w.info.runtime === 'failed')
  useEffect(() => {
    if (w.project || failed) setStarted(true)
  }, [w.project?.id, failed])
  return (
    <I18nProvider language={w.preferences.language}>
      <WorkspaceView workspace={w} />
      <StartupScreen ready={started || Boolean(w.project)} failed={failed} />
      <UpdateScreen
        ready={
          Boolean(w.project) && w.project?.connection === 'ready' && w.info.runtime === 'ready'
        }
        busy={w.busy || Boolean(w.info.automationCounts?.activeCount)}
        terminals={w.info.terminalCount || 0}
      />
    </I18nProvider>
  )
}

function WorkspaceView({ workspace: w }: { workspace: Workspace }) {
  const { t, locale } = useI18n()
  const documents = useDocuments(w.errorHandler)
  const [editorFraction, setEditorFraction] = useState(1.2)
  const [modal, setModal] = useState('')
  const [sidebar, setSidebar] = useState(true)
  const [inspector, setInspector] = useState(true)
  const [focusMode, setFocusMode] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [workbench, setWorkbench] = useState(false)
  const [workbenchExpanded, setWorkbenchExpanded] = useState(false)
  const [workbenchView, setWorkbenchView] = useState<WorkbenchView>('terminal')
  const showSidebar = sidebar && !focusMode
  const showInspector = inspector && !focusMode
  const showEditor = documents.documents.length > 0 && !focusMode
  useEffect(() => {
    setSidebar(w.preferences.sidebarVisible !== false)
    setInspector(w.preferences.inspectorVisible !== false)
  }, [w.preferences.sidebarVisible, w.preferences.inspectorVisible])
  useEffect(() => setSearchOpen(false), [w.session?.id])
  function toggleSidebar() {
    const next = !sidebar
    setSidebar(next)
    void w.savePreferences({ sidebarVisible: next }).catch(w.errorHandler)
  }
  function toggleInspector() {
    const next = !inspector
    setInspector(next)
    void w.savePreferences({ inspectorVisible: next }).catch(w.errorHandler)
  }
  const [browserRequest, setBrowserRequest] = useState(0)
  useEffect(
    () =>
      window.prime.onEvent((event) => {
        if (
          event.type === 'workbench' &&
          event.payload?.action === 'activate' &&
          event.workspaceId === w.project?.id
        ) {
          const view = event.payload.view
          if (['terminal', 'automations', 'teams', 'launch', 'github', 'windows'].includes(view)) {
            setWorkbenchView(view as WorkbenchView)
            setWorkbench(true)
            setFocusMode(false)
          }
        }
        if (
          event.type === 'windows' &&
          event.payload?.request &&
          event.workspaceId === w.project?.id
        ) {
          setWorkbenchView('windows')
          setWorkbench(true)
          setFocusMode(false)
        }
        if (event.type === 'browser' && event.payload?.action === 'activate') {
          setInspector(true)
          setFocusMode(false)
          setBrowserRequest(Date.now())
        }
      }),
    [w.project?.id]
  )
  const [widths, setWidths] = useState({ sidebar: 256, inspector: 334 })
  const [zoom, setZoom] = useState(1)
  const shell = useRef<HTMLDivElement>(null)
  const layoutWidth = useLayoutWidth(shell, zoom)
  const layout = panelGeometry(
    layoutWidth,
    widths.sidebar,
    widths.inspector,
    showSidebar,
    showInspector
  )
  useEffect(() => {
    const setup = () => setModal('providers')
    window.addEventListener('prime:providers', setup)
    return () => window.removeEventListener('prime:providers', setup)
  }, [])
  useEffect(() => {
    setWidths({ sidebar: w.preferences.sidebarWidth, inspector: w.preferences.inspectorWidth })
  }, [w.preferences.sidebarWidth, w.preferences.inspectorWidth])
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const dialogOpen = Boolean(document.querySelector('dialog[open]'))
      if (event.ctrlKey && event.code === 'KeyK') {
        event.preventDefault()
        if (!dialogOpen) setModal('commands')
        return
      }
      if (dialogOpen || event.defaultPrevented) return
      if (event.ctrlKey && event.code === 'Backquote' && w.project) {
        event.preventDefault()
        setWorkbench((value) => (workbenchView === 'terminal' ? !value : true))
        setWorkbenchView('terminal')
      }
      if (event.ctrlKey && event.shiftKey && event.code === 'KeyF') {
        event.preventDefault()
        setFocusMode((value) => !value)
      }
      if (
        event.ctrlKey &&
        !event.shiftKey &&
        event.code === 'KeyF' &&
        w.messages.length &&
        !(event.target as HTMLElement).closest('.file-editor, .terminal-panel')
      ) {
        event.preventDefault()
        setSearchOpen(true)
      }
      if (event.ctrlKey && event.code === 'Comma') {
        event.preventDefault()
        setModal('settings')
      }
      if (event.ctrlKey && event.code === 'KeyL') {
        event.preventDefault()
        window.dispatchEvent(new Event('prime:models'))
      }
      if (event.ctrlKey && event.code === 'KeyN') {
        event.preventDefault()
        if (w.project && !document.querySelector('dialog[open]')) void w.createSession()
      }
      if (event.ctrlKey && ['Equal', 'Minus', 'Digit0'].includes(event.code)) {
        event.preventDefault()
        setZoom((value) =>
          event.code === 'Digit0'
            ? 1
            : Math.max(0.8, Math.min(2, value + (event.code === 'Equal' ? 0.1 : -0.1)))
        )
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [w.project?.id, w.messages.length, workbenchView])
  function changePanelWidth(which: 'sidebar' | 'inspector', width: number, persist = false) {
    const next = Math.max(
      which === 'sidebar' ? 210 : 280,
      Math.min(which === 'sidebar' ? 400 : layout.inspectorMax, width)
    )
    setWidths((value) => ({ ...value, [which]: next }))
    if (persist)
      void w
        .savePreferences({ [which === 'sidebar' ? 'sidebarWidth' : 'inspectorWidth']: next })
        .catch(w.errorHandler)
    return next
  }
  function resize(which: 'sidebar' | 'inspector', event: React.PointerEvent<HTMLDivElement>) {
    event.preventDefault()
    event.currentTarget.setPointerCapture(event.pointerId)
    const start = event.clientX
    const initial = widths[which]
    let latest = initial
    const move = (move: PointerEvent) => {
      latest = changePanelWidth(
        which,
        initial + ((move.clientX - start) / zoom) * (which === 'sidebar' ? 1 : -1)
      )
    }
    const stop = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
      window.removeEventListener('pointercancel', stop)
      changePanelWidth(which, latest, true)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop)
    window.addEventListener('pointercancel', stop)
  }
  function resizeWithKeyboard(
    which: 'sidebar' | 'inspector',
    event: React.KeyboardEvent<HTMLDivElement>
  ) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const minimum = which === 'sidebar' ? 210 : 280
    const maximum = which === 'sidebar' ? 400 : layout.inspectorMax
    const direction = (event.key === 'ArrowRight' ? 1 : -1) * (which === 'sidebar' ? 1 : -1)
    changePanelWidth(
      which,
      event.key === 'Home'
        ? minimum
        : event.key === 'End'
          ? maximum
          : widths[which] + direction * (event.shiftKey ? 50 : 10),
      true
    )
  }
  const projectName = w.project?.kind === 'chat' ? t('Личные чаты') : projectLabel(w.project)
  const isEmpty = !w.messages.length && !w.busy
  const quickActions: QuickAction[] = [
    {
      id: 'terminal',
      label: t('Терминал проекта'),
      icon: TerminalSquare,
      shortcut: 'Ctrl `',
      disabled: !w.project,
      run: () => {
        setWorkbenchView('terminal')
        setWorkbench(true)
      }
    },
    {
      id: 'jobs',
      label: t('Фоновые задачи'),
      icon: Clock3,
      disabled: w.info.runtime !== 'ready',
      run: () => setModal('jobs')
    },
    {
      id: 'teams',
      label: t('Команды агентов'),
      icon: Users,
      disabled: !w.project,
      run: () => {
        setWorkbenchView('teams')
        setWorkbench(true)
      }
    },
    {
      id: 'automations',
      label: t('Автоматизации'),
      icon: Clock3,
      disabled: !w.project,
      run: () => {
        setWorkbenchView('automations')
        setWorkbench(true)
      }
    },
    {
      id: 'new',
      label: t('Новый чат'),
      icon: Plus,
      shortcut: 'Ctrl N',
      disabled: !w.project,
      run: () => {
        void w.createSession()
      }
    },
    {
      id: 'open',
      label: t('Открыть проект'),
      icon: PanelRightOpen,
      disabled: w.info.runtime !== 'ready',
      run: () => {
        void w.openProject()
      }
    },
    {
      id: 'remote',
      label: t('SSH-серверы'),
      icon: Server,
      disabled: w.info.runtime !== 'ready',
      run: () => setModal('remote')
    },
    {
      id: 'models',
      label: t('Выбрать модель'),
      icon: Command,
      shortcut: 'Ctrl L',
      run: () => {
        window.dispatchEvent(new Event('prime:models'))
      }
    },
    {
      id: 'find',
      label: t('Поиск в чате'),
      icon: Search,
      shortcut: 'Ctrl F',
      disabled: !w.messages.length,
      run: () => setSearchOpen(true)
    },
    {
      id: 'focus',
      label: focusMode ? t('Выйти из фокуса') : t('Режим фокуса'),
      icon: Focus,
      shortcut: 'Ctrl Shift F',
      run: () => setFocusMode((value) => !value)
    },
    {
      id: 'sidebar',
      label: sidebar ? t('Скрыть проекты') : t('Показать проекты'),
      icon: PanelLeftOpen,
      run: toggleSidebar
    },
    {
      id: 'inspector',
      label: inspector ? t('Скрыть рабочую область') : t('Показать рабочую область'),
      icon: PanelRightOpen,
      run: toggleInspector
    },
    {
      id: 'theme',
      label: t('Переключить тему'),
      icon: SunMoon,
      run: () => {
        void w
          .savePreferences({
            theme: document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark'
          })
          .catch(w.errorHandler)
      }
    },
    {
      id: 'export',
      label: t('Экспорт в Markdown'),
      icon: Download,
      disabled: !w.messages.length,
      run: () => downloadConversation(w.session?.title || t('Новый чат'), w.messages, t('Вы'))
    },
    { id: 'plugins', label: t('Плагины'), icon: Puzzle, run: () => setModal('plugins') },
    {
      id: 'settings',
      label: t('Настройки'),
      icon: Settings2,
      shortcut: 'Ctrl ,',
      run: () => setModal('settings')
    }
  ]
  return (
    <>
      <Titlebar />
      <ProjectLockBanner />
      <WindowControlBadge
        workspaceIds={w.projects.map((project) => project.id)}
        onError={w.errorHandler}
      />
      <div
        ref={shell}
        className={`app-shell ${showSidebar ? '' : 'hide-sidebar'} ${showInspector ? '' : 'hide-inspector'} ${showEditor ? 'has-editor' : ''} ${focusMode ? 'focus-mode' : ''} ${layout.compact ? 'compact-layout' : ''}`}
        style={
          {
            '--sidebar-width': widths.sidebar + 'px',
            '--editor-fraction': `${editorFraction}fr`,
            '--inspector-width': widths.inspector + 'px',
            '--layout-zoom': zoom,
            zoom,
            gridTemplateColumns:
              layout.columns +
              (showEditor && !layout.compact ? ' 1px minmax(0, var(--editor-fraction))' : '')
          } as React.CSSProperties
        }
      >
        {showSidebar && (
          <>
            {layout.sidebarCompact ? (
              <ProjectRail
                workspace={w}
                onCommands={() => setModal('commands')}
                onSettings={() => setModal('settings')}
              >
                <Sidebar
                  workspace={w}
                  onSettings={() => setModal('settings')}
                  onPlugins={() => setModal('plugins')}
                  onCommands={() => setModal('commands')}
                  onRemote={() => setModal('remote')}
                  onJobs={() => setModal('jobs')}
                  onWorkbench={() => setWorkbench(true)}
                />
              </ProjectRail>
            ) : (
              <Sidebar
                workspace={w}
                onSettings={() => setModal('settings')}
                onPlugins={() => setModal('plugins')}
                onCommands={() => setModal('commands')}
                onRemote={() => setModal('remote')}
                onJobs={() => setModal('jobs')}
                onWorkbench={() => setWorkbench(true)}
              />
            )}
            {!layout.sidebarCompact && (
              <div
                className="resizer left-resizer"
                role="separator"
                aria-label={t('Ширина панели проектов')}
                tabIndex={0}
                onPointerDown={(event) => resize('sidebar', event)}
                aria-orientation="vertical"
                aria-valuemin={210}
                aria-valuemax={400}
                aria-valuenow={layout.sidebar}
                onKeyDown={(event) => resizeWithKeyboard('sidebar', event)}
              />
            )}
          </>
        )}
        <main className="main-workspace" data-workbench-expanded={workbench && workbenchExpanded}>
          <header className="workspace-header">
            <button
              className="icon"
              title={showSidebar ? t('Скрыть проекты') : t('Показать проекты')}
              aria-label={t('Панель проектов')}
              onClick={() => {
                if (focusMode) setFocusMode(false)
                else toggleSidebar()
              }}
            >
              {showSidebar ? <PanelLeftClose size={17} /> : <PanelLeftOpen size={17} />}
            </button>
            <div className="breadcrumbs">
              <span>{projectName || t('Рабочее пространство')}</span>
              <ChevronRight size={13} />
              <strong>{w.session?.title || t('Новый чат')}</strong>
            </div>
            <div className="header-actions">
              <span
                className={`connection-status ${w.project?.connection === 'reconnecting' ? 'amber-text' : ''}`}
              >
                <span className={`dot ${w.info.runtime === 'ready' ? 'green' : 'amber'}`} />
                {w.project?.connection === 'reconnecting'
                  ? t('Переподключение')
                  : w.info.runtime === 'ready'
                    ? t('Готов к работе')
                    : t('Запускаем Prime')}
              </span>
              <button
                className="icon"
                title={t('Быстрые действия') + ' · Ctrl K'}
                aria-label={t('Быстрые действия')}
                onClick={() => setModal('commands')}
              >
                <Command size={17} />
              </button>
              <button
                className={`icon ${focusMode ? 'active-control' : ''}`}
                title={t('Режим фокуса') + ' · Ctrl Shift F'}
                aria-label={focusMode ? t('Выйти из фокуса') : t('Режим фокуса')}
                aria-pressed={focusMode}
                onClick={() => setFocusMode((value) => !value)}
              >
                <Focus size={17} />
              </button>
              {w.messages.length > 0 && (
                <button
                  className={`icon ${searchOpen ? 'active-control' : ''}`}
                  title={t('Поиск в чате') + ' · Ctrl F'}
                  aria-label={t('Поиск в чате')}
                  aria-expanded={searchOpen}
                  onClick={() => setSearchOpen((value) => !value)}
                >
                  <Search size={17} />
                </button>
              )}
              <ChatActions workspace={w} />
              <button
                className={`icon ${workbench ? 'active-control' : ''}`}
                title={t('Терминал проекта') + ' · Ctrl `'}
                aria-label={t('Терминал проекта')}
                aria-expanded={workbench}
                disabled={!w.project}
                onClick={() => {
                  setWorkbench((value) => (workbenchView === 'terminal' ? !value : true))
                  setWorkbenchView('terminal')
                }}
              >
                <TerminalSquare size={17} />
              </button>
              {!showInspector && (
                <button
                  className="icon"
                  aria-label={t('Показать рабочую область')}
                  title={t('Рабочая область')}
                  onClick={() => {
                    setFocusMode(false)
                    setInspector(true)
                    void w.savePreferences({ inspectorVisible: true }).catch(w.errorHandler)
                  }}
                >
                  <PanelRightOpen size={17} />
                </button>
              )}
            </div>
          </header>
          {(w.error || w.info.error) && (
            <div className="error-banner" role="alert">
              <AlertCircle size={16} />
              <span>{w.error || w.info.error}</span>
              <button onClick={() => setModal('providers')}>{t('Настройки')}</button>
              <button
                className="icon small"
                aria-label={t('Скрыть ошибку')}
                onClick={() => w.setError('')}
              >
                <X size={14} />
              </button>
            </div>
          )}
          {isEmpty ? (
            <Welcome workspace={w} openCommands={() => setModal('commands')} />
          ) : (
            <MessageList
              key={w.session?.id || 'new'}
              messages={w.messages}
              busy={w.busy}
              searchOpen={searchOpen}
              closeSearch={() => setSearchOpen(false)}
              waiting={
                w.permissions.some((item) => item.session_id === w.session?.id) ||
                w.questions.some((item) => item.session_id === w.session?.id)
              }
            />
          )}
          <Approvals workspace={w} />
          <Composer workspace={w} />
          {workbench && (
            <WorkbenchDock
              view={workbenchView}
              setView={setWorkbenchView}
              workspace={w}
              expanded={workbenchExpanded}
              setExpanded={setWorkbenchExpanded}
              close={() => setWorkbench(false)}
            />
          )}
          <footer className="workspace-statusbar">
            <span>
              <GitBranch size={12} />
              {projectName || t('Проект не выбран')}
            </span>
            <span>
              {w.session
                ? t('{value0} токенов · ${value1}', {
                    value0: (w.session.prompt_tokens || 0).toLocaleString(locale),
                    value1: (w.session.cost || 0).toLocaleString(locale, {
                      minimumFractionDigits: 4,
                      maximumFractionDigits: 4
                    })
                  })
                : t('Всё необходимое — в одном пространстве')}
              <span className="status-separator" />
              Prime Desktop
            </span>
          </footer>
        </main>
        {showInspector && (
          <>
            {!layout.compact && (
              <div
                className="resizer right-resizer"
                role="separator"
                aria-label={t('Ширина рабочей области')}
                tabIndex={0}
                onPointerDown={(event) => resize('inspector', event)}
                aria-orientation="vertical"
                aria-valuemin={280}
                aria-valuemax={layout.inspectorMax}
                aria-valuenow={layout.inspector}
                onKeyDown={(event) => resizeWithKeyboard('inspector', event)}
              />
            )}
            <Inspector
              compact={layout.compact}
              workspace={w}
              close={toggleInspector}
              openDocument={documents.open}
              browserRequest={browserRequest}
            />
          </>
        )}
        {documents.documents.length > 0 && (
          <>
            <div
              className="resizer editor-divider"
              role="separator"
              aria-label={t('Ширина редактора')}
              onPointerDown={(event) => {
                event.preventDefault()
                const start = event.clientX
                const editorWidth =
                  shell.current!.querySelector<HTMLElement>('.file-editor')!.offsetWidth
                const available =
                  editorWidth +
                  shell.current!.querySelector<HTMLElement>('.main-workspace')!.offsetWidth
                const move = (event: PointerEvent) => {
                  const width = Math.max(
                    360,
                    Math.min(available - 300, editorWidth - (event.clientX - start))
                  )
                  setEditorFraction(width / (available - width))
                }
                const stop = () => {
                  window.removeEventListener('pointermove', move)
                  window.removeEventListener('pointerup', stop)
                }
                window.addEventListener('pointermove', move)
                window.addEventListener('pointerup', stop)
              }}
            />
            <Suspense fallback={<div className="editor-loading">{t('Открываем редактор…')}</div>}>
              <FileEditor state={documents} theme={w.preferences.theme} enabled={!focusMode} />
            </Suspense>
          </>
        )}
        {modal === 'commands' && (
          <CommandPalette
            actions={quickActions}
            sessions={w.sessions}
            selectSession={(session) => {
              void w.selectSession(session).catch(w.errorHandler)
            }}
            close={() => setModal('')}
          />
        )}
        {(modal === 'settings' || modal === 'providers') && (
          <SettingsDialog
            workspace={w}
            close={() => setModal('')}
            initial={modal === 'providers' ? 'providers' : 'appearance'}
          />
        )}
        {modal === 'plugins' && <PluginManager workspace={w} close={() => setModal('')} />}
        {modal === 'remote' && <RemoteDialog workspace={w} close={() => setModal('')} />}
        {modal === 'jobs' && (
          <JobsCenter
            workspace={w}
            close={() => setModal('')}
            openTools={(view) => {
              setWorkbenchView(view)
              setWorkbench(true)
            }}
          />
        )}
      </div>
    </>
  )
}

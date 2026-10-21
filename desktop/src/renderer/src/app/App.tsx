import { chatCenterShift, panelGeometry, panelGutter } from './panelGeometry'
import { useLayoutWidth } from './useLayoutWidth'
import { ProjectRail } from '../features/projects/ProjectRail'
import { projectName as projectLabel } from '../features/projects/projectName'
import { useDocuments } from '../features/files/useDocuments'
import { StartupScreen } from '../components/StartupScreen'
import { UpdateScreen } from '../features/updates/UpdateScreen'
import { Titlebar } from '../components/Titlebar'
import { I18nProvider, useI18n } from '../i18n'
import { lazy, Suspense, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { AlertCircle, X } from 'lucide-react'
import { useWorkspace, type Workspace } from './useWorkspace'
import { Sidebar } from '../features/projects/Sidebar'
import { MessageList } from '../features/chat/MessageList'
import { Composer } from '../features/chat/Composer'
import { Approvals } from '../features/approvals/Approvals'
import { Inspector } from '../features/files/Inspector'
import { SettingsDialog } from '../features/settings/SettingsDialog'
import { PluginManager } from '../features/plugins/PluginManager'
import { Welcome } from '../features/projects/Welcome'
import { CommandPalette } from '../features/navigation/CommandPalette'
import { WorkbenchDock, type WorkbenchView } from '../features/workbench/WorkbenchDock'
import { RemoteDialog } from '../features/remote/RemoteDialog'
import { JobsCenter } from '../features/jobs/JobsCenter'
import { ProjectLockBanner } from '../features/projects/ProjectLockBanner'
import { WindowControlBadge } from '../features/windows/WindowControlBadge'
import { WorkspaceTopbar } from './WorkspaceTopbar'
import { useQuickActions } from './useQuickActions'
import { usePanelResize } from './usePanelResize'
import { useWorkspaceShortcuts } from './useWorkspaceShortcuts'
import { useModalWindowChrome } from './useModalWindowChrome'
import { applyShortcuts } from '../features/shortcuts/shortcutStore'

const FileEditor = lazy(() => import('../features/files/FileEditor'))

export function App() {
  const w = useWorkspace()
  const [started, setStarted] = useState(false)
  const failed = Boolean(w.error || w.info.error || w.info.runtime === 'failed')
  const startupNeedsInput = w.questions.length > 0 || w.permissions.length > 0
  const ready = started || w.startupReady || startupNeedsInput
  useEffect(() => {
    if (ready || failed) setStarted(true)
  }, [ready, failed])
  return (
    <I18nProvider language={w.preferences.language}>
      <StartupScreen ready={ready} failed={failed}>
        <WorkspaceView workspace={w} />
      </StartupScreen>
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
  const { t } = useI18n()
  const documents = useDocuments(w.errorHandler)
  useLayoutEffect(() => applyShortcuts(w.preferences.keybindings), [w.preferences.keybindings])
  useModalWindowChrome()
  const [editorFraction, setEditorFraction] = useState(1.2)
  const [modal, setModal] = useState('')
  const [collapsed, setCollapsedState] = useState({ sidebar: true, inspector: true })
  const [inspector, setInspector] = useState(true)
  const [focusMode, setFocusMode] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [workbench, setWorkbench] = useState(false)
  const [workbenchExpanded, setWorkbenchExpanded] = useState(false)
  const [workbenchView, setWorkbenchView] = useState<WorkbenchView>('terminal')
  const showSidebar = !focusMode
  const showInspector = inspector && !focusMode
  const showEditor = documents.documents.length > 0 && !focusMode
  useEffect(() => setSearchOpen(false), [w.session?.id])
  function setCollapsed(which: 'sidebar' | 'inspector', value: boolean) {
    setCollapsedState((current) =>
      current[which] === value ? current : { ...current, [which]: value }
    )
  }
  function toggleSidebar() {
    if (focusMode) return setFocusMode(false)
    if (layout.compact) return window.dispatchEvent(new Event('prime:projects-flyout'))
    setCollapsed('sidebar', !collapsed.sidebar)
  }
  function toggleInspector() {
    if (focusMode) return setFocusMode(false)
    setInspector(true)
    if (layout.compact) return window.dispatchEvent(new Event('prime:inspector-panel'))
    setCollapsed('inspector', !collapsed.inspector)
  }
  function openTools(view: WorkbenchView) {
    setWorkbenchView(view)
    setWorkbench(true)
    setFocusMode(false)
  }
  function toggleTerminal() {
    setWorkbench((value) => (workbenchView === 'terminal' ? !value : true))
    setWorkbenchView('terminal')
  }
  const toggleFocus = () => setFocusMode((value) => !value)
  const [browserRequest, setBrowserRequest] = useState(0)
  useEffect(
    () =>
      window.prime.onEvent((event) => {
        if (
          event.type === 'workbench' &&
          event.payload?.action === 'activate' &&
          event.workspaceId === w.project?.id &&
          ['terminal', 'automations', 'teams', 'launch', 'github', 'windows'].includes(
            event.payload.view
          )
        )
          openTools(event.payload.view as WorkbenchView)
        if (
          event.type === 'windows' &&
          event.payload?.request &&
          event.workspaceId === w.project?.id
        )
          openTools('windows')
        if (event.type === 'browser' && event.payload?.action === 'activate') {
          setInspector(true)
          setCollapsed('inspector', false)
          setFocusMode(false)
          setBrowserRequest(Date.now())
          if (event.payload.agent) widenForAgent(event.payload.width)
        }
      }),
    [w.project?.id]
  )
  const [widths, setWidths] = useState({ sidebar: 256, inspector: 334 })
  const [zoom, setZoom] = useState(1)
  const shell = useRef<HTMLDivElement>(null)
  const layoutWidth = useLayoutWidth(shell, zoom)
  const layout = panelGeometry(
    layoutWidth - panelGutter * 2,
    widths.sidebar,
    widths.inspector,
    showSidebar,
    showInspector,
    collapsed.sidebar,
    collapsed.inspector
  )
  const centerShift = chatCenterShift(layout, showSidebar, showInspector, showEditor)
  const geometry = useRef({ available: 0, inspectorMax: 280 })
  geometry.current = { available: layoutWidth - panelGutter * 2, inspectorMax: layout.inspectorMax }
  function widenForAgent(requested?: number) {
    const { available, inspectorMax } = geometry.current
    setWidths((value) => {
      const target = requested ?? Math.max(value.inspector, Math.round(available * 0.55))
      const inspector = Math.max(280, Math.min(inspectorMax, target))
      return inspector === value.inspector ? value : { ...value, inspector }
    })
  }
  const panels = usePanelResize({
    workspace: w,
    widths,
    setWidths,
    collapsed,
    setCollapsed,
    inspectorMax: layout.inspectorMax,
    zoom
  })
  useEffect(() => {
    const setup = () => setModal('providers')
    window.addEventListener('prime:providers', setup)
    return () => window.removeEventListener('prime:providers', setup)
  }, [])
  useEffect(() => {
    setWidths({ sidebar: w.preferences.sidebarWidth, inspector: w.preferences.inspectorWidth })
  }, [w.preferences.sidebarWidth, w.preferences.inspectorWidth])
  useWorkspaceShortcuts({
    workspace: w,
    setModal,
    toggleTerminal,
    toggleFocus,
    openSearch: () => setSearchOpen(true),
    setZoom
  })
  const projectName = w.project?.kind === 'chat' ? t('Личные чаты') : projectLabel(w.project)
  const isEmpty = !w.messages.length && !w.busy && w.messagesReady
  const quickActions = useQuickActions({
    workspace: w,
    openTools,
    setModal,
    openSearch: () => setSearchOpen(true),
    focusMode,
    toggleFocus,
    sidebarVisible: !collapsed.sidebar,
    toggleSidebar,
    inspectorVisible: inspector && !collapsed.inspector,
    toggleInspector
  })
  const sidebarPanel = (
    <Sidebar
      workspace={w}
      onSettings={() => setModal('settings')}
      onPlugins={() => setModal('plugins')}
      onRemote={() => setModal('remote')}
      onJobs={() => setModal('jobs')}
      onWorkbench={() => openTools(workbenchView)}
    />
  )
  return (
    <>
      <Titlebar>
        <WorkspaceTopbar
          workspace={w}
          projectName={projectName || ''}
          sidebarVisible={showSidebar && layout.sidebarOpen}
          toggleSidebar={toggleSidebar}
          inspectorVisible={showInspector && layout.inspectorOpen}
          toggleInspector={toggleInspector}
          focusMode={focusMode}
          toggleFocus={toggleFocus}
          searchOpen={searchOpen}
          toggleSearch={() => setSearchOpen((value) => !value)}
          toolsOpen={workbench}
          toggleTools={toggleTerminal}
          openCommands={() => setModal('commands')}
        />
      </Titlebar>
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
            '--center-shift': centerShift + 'px',
            '--layout-zoom': zoom,
            zoom,
            gridTemplateColumns:
              layout.columns +
              (showEditor && !layout.compact ? ` ${panelGutter}px minmax(0, var(--editor-fraction))` : '')
          } as React.CSSProperties
        }
      >
        {showSidebar && (
          <>
            <ProjectRail
              workspace={w}
              onCommands={() => setModal('commands')}
              onSettings={() => setModal('settings')}
              onJobs={() => setModal('jobs')}
              onTools={() => openTools(workbenchView)}
              onPlugins={() => setModal('plugins')}
              expanded={layout.sidebarOpen}
              onToggle={layout.compact ? undefined : toggleSidebar}
            >
              {sidebarPanel}
            </ProjectRail>
            {layout.sidebarOpen && sidebarPanel}
            {layout.sidebarResizable && (
              <div {...panels.separator('sidebar', t('Ширина панели проектов'), layout.sidebar)} />
            )}
          </>
        )}
        <main className="main-workspace" data-workbench-expanded={workbench && workbenchExpanded}>
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
        </main>
        {showInspector && (
          <>
            {layout.inspectorResizable && (
              <div {...panels.separator('inspector', t('Ширина рабочей области'), layout.inspector)} />
            )}
            <Inspector
              compact={layout.compact}
              expanded={layout.inspectorOpen}
              setExpanded={(open) => setCollapsed('inspector', !open)}
              workspace={w}
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
            openTools={openTools}
          />
        )}
      </div>
    </>
  )
}

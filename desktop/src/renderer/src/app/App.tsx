import { PanelBoundary } from '../components/PanelBoundary'
import { CreateProjectDialog } from '../features/projects/CreateProjectDialog'
import { chatCenterShift, panelGeometry, panelGutter } from './panelGeometry'
import { useLayoutWidth } from './useLayoutWidth'
import { ProjectRail } from '../features/projects/ProjectRail'
import { projectName as projectLabel } from '../features/projects/projectName'
import { useDocuments } from '../features/files/useDocuments'
import { StartupScreen } from '../components/StartupScreen'
import { UpdateScreen } from '../features/updates/UpdateScreen'
import { Titlebar } from '../components/Titlebar'
import { I18nProvider, translate, useI18n } from '../i18n'
import { lazy, Suspense, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { ToastStack } from '../features/notifications/ToastStack'
import { notifyError } from '../features/notifications/notifications'
import { useWorkspace, type Workspace } from './useWorkspace'
import { Sidebar } from '../features/projects/Sidebar'
import { HistoryMessageList } from '../features/chat/HistoryMessageList'
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
import { WorkspaceTopbar } from './WorkspaceTopbar'
import { useQuickActions } from './useQuickActions'
import { usePanelResize } from './usePanelResize'
import { useWorkspaceShortcuts } from './useWorkspaceShortcuts'
import { useModalWindowChrome } from './useModalWindowChrome'
import { applyShortcuts } from '../features/shortcuts/shortcutStore'
import { openAgentEvent, type AgentRequest } from '../features/agents/openAgent'
import { isForViewedChat, useBrowserView } from '../features/browser/useBrowserView'
import { isInspectorOpen, shouldActivateBrowserTab } from '../features/browser/browserActivation'
import { realSessionId } from '../../../shared/pendingSession'

const FileEditor = lazy(() => import('../features/files/FileEditor'))

export function App() {
  const w = useWorkspace()
  const languageRef = useRef(w.preferences.language)
  languageRef.current = w.preferences.language
  useEffect(() => {
    const handleHostError = (event: Event) => {
      const detail: unknown = (event as CustomEvent<unknown>).detail
      w.errorHandler(
        detail instanceof Error
          ? detail
          : new Error(
              typeof detail === 'string'
                ? detail
                : (detail as { message?: string })?.message ||
                    translate(languageRef.current, 'Ошибка окна')
            )
      )
    }
    window.addEventListener('prime:host-error', handleHostError)
    return () => window.removeEventListener('prime:host-error', handleHostError)
  }, [w.errorHandler])
  useEffect(() => {
    if (!w.error) return
    notifyError(w.error)
    w.setError('')
  }, [w.error])
  useEffect(() => {
    if (w.info.error) notifyError(w.info.error)
  }, [w.info.error])
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
  const [settingsTab, setSettingsTab] = useState('appearance')
  useEffect(() => {
    const openSettings = (event: Event) => {
      const tab = (event as CustomEvent<{ tab?: string }>).detail?.tab
      setSettingsTab(tab || 'appearance')
      setModal('settings')
    }
    window.addEventListener('prime:open-settings', openSettings)
    return () => window.removeEventListener('prime:open-settings', openSettings)
  }, [])
  const [collapsed, setCollapsedState] = useState({ sidebar: true, inspector: true })
  const [inspector, setInspector] = useState(true)
  const [focusMode, setFocusMode] = useState(false)
  const [compactPanelOpen, setCompactPanelOpen] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [workbench, setWorkbench] = useState(false)
  const [workbenchExpanded, setWorkbenchExpanded] = useState(false)
  const [workbenchView, setWorkbenchView] = useState<WorkbenchView>('terminal')
  const showSidebar = !focusMode
  const showInspector = inspector && !focusMode
  const showEditor = documents.documents.length > 0 && !focusMode
  useEffect(() => setSearchOpen(false), [w.session?.id])
  function setCollapsed(which: 'sidebar' | 'inspector', value: boolean, persist = false) {
    setCollapsedState((current) =>
      current[which] === value ? current : { ...current, [which]: value }
    )
    if (persist) {
      void w
        .savePreferences({ [which === 'sidebar' ? 'sidebarVisible' : 'inspectorVisible']: !value })
        .catch(w.errorHandler)
    }
  }
  function toggleSidebar() {
    if (focusMode) return setFocusMode(false)
    if (layout.compact) return window.dispatchEvent(new Event('prime:projects-flyout'))
    if (!layout.sidebarOpen) {
      setWidths((value) => ({
        ...value,
        inspector: Math.min(value.inspector, layout.inspectorWithSidebarMax)
      }))
      setCollapsed('sidebar', false, true)
      return
    }
    setCollapsed('sidebar', !collapsed.sidebar, true)
  }
  function keepSidebarBesideInspector() {
    const current = layoutRef.current
    if (!current.sidebarOpen) return
    setWidths((value) =>
      value.inspector <= current.inspectorWithSidebarMax
        ? value
        : { ...value, inspector: current.inspectorWithSidebarMax }
    )
  }
  function toggleInspector() {
    if (focusMode) return setFocusMode(false)
    setInspector(true)
    if (layout.compact) {
      setCompactPanelOpen((value) => !value)
      return
    }
    if (collapsed.inspector) keepSidebarBesideInspector()
    setCollapsed('inspector', !collapsed.inspector, true)
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
    collapsed.inspector,
    showEditor
  )
  const layoutRef = useRef(layout)
  layoutRef.current = layout
  useEffect(() => {
    if (showEditor) keepSidebarBesideInspector()
  }, [showEditor])
  const centerShift = chatCenterShift(layout, showSidebar, showInspector, showEditor)
  const inspectorOpen = isInspectorOpen(
    layout.compact,
    compactPanelOpen,
    showInspector,
    collapsed.inspector
  )
  const inspectorOpenRef = useRef(inspectorOpen)
  inspectorOpenRef.current = inspectorOpen
  const [browserRequest, setBrowserRequest] = useState(0)
  const [appRequest, setAppRequest] = useState(0)
  const viewedChat = useBrowserView(w.session, inspectorOpen)
  const [agentRequest, setAgentRequest] = useState<AgentRequest>()
  const sessionRef = useRef(w.session)
  sessionRef.current = w.session
  useEffect(() => {
    setAgentRequest(undefined)
  }, [w.session?.id])
  useEffect(() => {
    const open = (event: Event) => {
      const detail = (event as CustomEvent<{ workerId?: string; sessionId?: string }>).detail
      if (!detail?.workerId) return
      setFocusMode(false)
      setInspector(true)
      setAgentRequest((current) => ({
        workerId: detail.workerId!,
        sessionId: detail.sessionId ?? realSessionId(sessionRef.current),
        sequence: (current?.sequence || 0) + 1
      }))
    }
    window.addEventListener(openAgentEvent, open)
    return () => window.removeEventListener(openAgentEvent, open)
  }, [])
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
        if (
          event.type === 'appui' &&
          event.payload?.activate &&
          event.workspaceId === w.project?.id &&
          isForViewedChat(event.payload.sessionId, viewedChat.current)
        ) {
          setInspector(true)
          keepSidebarBesideInspector()
          setCollapsed('inspector', false)
          setFocusMode(false)
          setAppRequest(Date.now())
        }
        if (
          event.type === 'browser' &&
          shouldActivateBrowserTab(event.payload, viewedChat.current, inspectorOpenRef.current)
        ) {
          setBrowserRequest(Date.now())
        }
      }),
    [w.project?.id]
  )
  const panels = usePanelResize({
    workspace: w,
    widths,
    setWidths,
    collapsed,
    setCollapsed,
    inspectorMax: layout.sidebarOpen
      ? Math.min(layout.inspectorMax, layout.inspectorWithSidebarMax)
      : layout.inspectorMax,
    zoom,
    beginInspectorResize: () => {
      const chatWidth = shell.current?.querySelector<HTMLElement>('.main-workspace')?.offsetWidth
      const editorWidth = shell.current?.querySelector<HTMLElement>('.file-editor')?.offsetWidth
      const inspectorWidth = layout.inspector
      return (next) => {
        if (!showEditor || layout.compact || !chatWidth || !editorWidth) return
        const releasedGutter = (inspectorWidth > 0 ? panelGutter : 0) - (next > 0 ? panelGutter : 0)
        setEditorFraction(
          Math.max(300, editorWidth + inspectorWidth - next + releasedGutter) / chatWidth
        )
      }
    }
  })
  useEffect(() => {
    const setup = () => setModal('providers')
    window.addEventListener('prime:providers', setup)
    return () => window.removeEventListener('prime:providers', setup)
  }, [])
  useEffect(() => {
    setWidths({ sidebar: w.preferences.sidebarWidth, inspector: w.preferences.inspectorWidth })
  }, [w.preferences.sidebarWidth, w.preferences.inspectorWidth])
  useEffect(() => {
    setCollapsedState((current) => ({
      sidebar:
        w.preferences.sidebarVisible === undefined ? current.sidebar : !w.preferences.sidebarVisible,
      inspector:
        w.preferences.inspectorVisible === undefined
          ? current.inspector
          : !w.preferences.inspectorVisible
    }))
  }, [w.preferences.sidebarVisible, w.preferences.inspectorVisible])
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
    inspectorVisible: inspectorOpen,
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
      {w.projectCreationOpen && <CreateProjectDialog workspace={w} />}
      <ToastStack openProviders={() => setModal('providers')} />
      <Titlebar>
        <WorkspaceTopbar
          workspace={w}
          projectName={projectName || ''}
          sidebarVisible={showSidebar && layout.sidebarOpen}
          toggleSidebar={toggleSidebar}
          inspectorVisible={inspectorOpen}
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
              (showEditor && !layout.compact
                ? ` ${panelGutter}px minmax(0, var(--editor-fraction))`
                : '')
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
              compact={layout.compact}
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
          <PanelBoundary resetKey={w.session?.id} compact={layout.compact}>
            {isEmpty ? (
              <Welcome workspace={w} openCommands={() => setModal('commands')} />
            ) : (
              <HistoryMessageList
                key={w.session?.id || 'new'}
                history={w.history}
                busy={w.busy}
                retrying={w.retrying && w.busy}
                historyLoading={Boolean(w.history.loading) || (!w.messagesReady && !w.history.error)}
                historyError={w.history.error}
                loadHistory={w.loadHistory}
                searchMessages={w.searchMessages}
                revealMessage={w.revealMessage}
                searchOpen={searchOpen}
                closeSearch={() => setSearchOpen(false)}
                waiting={
                  w.permissions.some((item) => item.session_id === w.session?.id) ||
                  w.questions.some((item) => item.session_id === w.session?.id)
                }
              />
            )}
          </PanelBoundary>
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
              <div
                {...panels.separator('inspector', t('Ширина рабочей области'), layout.inspector)}
              />
            )}
            <Inspector
              compact={layout.compact}
              compactOpen={compactPanelOpen}
              setCompactOpen={setCompactPanelOpen}
              expanded={layout.inspectorOpen}
              setExpanded={(open) => {
                if (open) keepSidebarBesideInspector()
                setCollapsed('inspector', !open, true)
              }}
              workspace={w}
              openDocument={documents.open}
              browserRequest={browserRequest}
              appRequest={appRequest}
              agentRequest={agentRequest}
            />
          </>
        )}
        {documents.documents.length > 0 && (
          <>
            <div
              className="resizer editor-divider"
              role="separator"
              aria-orientation="vertical"
              aria-label={t('Ширина редактора')}
              aria-valuemin={10}
              aria-valuemax={90}
              aria-valuenow={Math.round((editorFraction / (1 + editorFraction)) * 100)}
              tabIndex={0}
              onKeyDown={(event) => {
                if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
                event.preventDefault()
                const editorWidth =
                  shell.current?.querySelector<HTMLElement>('.file-editor')?.offsetWidth
                const mainWidth =
                  shell.current?.querySelector<HTMLElement>('.main-workspace')?.offsetWidth
                if (!editorWidth || !mainWidth) return
                const available = editorWidth + mainWidth
                const step = event.shiftKey ? 50 : 10
                const targetWidth =
                  event.key === 'Home'
                    ? 300
                    : event.key === 'End'
                      ? available - 300
                      : editorWidth + (event.key === 'ArrowLeft' ? step : -step)
                const width = Math.max(300, Math.min(available - 300, targetWidth))
                setEditorFraction(width / Math.max(300, available - width))
              }}
              onPointerDown={(event) => {
                event.preventDefault()
                const target = event.currentTarget
                const pointerId = event.pointerId
                target.setPointerCapture?.(pointerId)
                const start = event.clientX
                const editorWidth =
                  shell.current!.querySelector<HTMLElement>('.file-editor')!.offsetWidth
                const available =
                  editorWidth +
                  shell.current!.querySelector<HTMLElement>('.main-workspace')!.offsetWidth
                const move = (event: PointerEvent) => {
                  const width = Math.max(
                    300,
                    Math.min(available - 300, editorWidth - (event.clientX - start) / zoom)
                  )
                  setEditorFraction(width / (available - width))
                }
                const stop = () => {
                  window.removeEventListener('pointermove', move)
                  window.removeEventListener('pointerup', stop)
                  window.removeEventListener('pointercancel', stop)
                  if (target.hasPointerCapture?.(pointerId)) target.releasePointerCapture(pointerId)
                }
                window.addEventListener('pointermove', move)
                window.addEventListener('pointerup', stop)
                window.addEventListener('pointercancel', stop)
              }}
            />
            <Suspense fallback={<div className="editor-loading">{t('Открываем редактор…')}</div>}>
              <FileEditor state={documents} theme={w.preferences.theme} enabled={!focusMode} />
            </Suspense>
          </>
        )}
        <PanelBoundary resetKey={modal}>
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
              initial={modal === 'providers' ? 'providers' : settingsTab}
            />
          )}
          {modal === 'plugins' && <PluginManager workspace={w} close={() => setModal('')} />}
          {modal === 'remote' && <RemoteDialog workspace={w} close={() => setModal('')} />}
          {modal === 'jobs' && (
            <JobsCenter workspace={w} close={() => setModal('')} openTools={openTools} />
          )}
        </PanelBoundary>
      </div>
    </>
  )
}

import { shortcutLabel } from '../../../shared/keybindings'
import { useShortcuts } from '../features/shortcuts/shortcutStore'
import {
  ChevronRight,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
  Search,
  TextSearch,
  TerminalSquare
} from 'lucide-react'
import { useI18n } from '../i18n'
import { ChatActions } from '../features/chat/ChatActions'
import type { Workspace } from './useWorkspace'

export function WorkspaceTopbar({
  workspace: w,
  projectName,
  sidebarVisible,
  toggleSidebar,
  inspectorVisible,
  toggleInspector,
  focusMode,
  toggleFocus,
  searchOpen,
  toggleSearch,
  toolsOpen,
  toggleTools,
  openCommands
}: {
  workspace: Workspace
  projectName: string
  sidebarVisible: boolean
  toggleSidebar: () => void
  inspectorVisible: boolean
  toggleInspector: () => void
  focusMode: boolean
  toggleFocus: () => void
  searchOpen: boolean
  toggleSearch: () => void
  toolsOpen: boolean
  toggleTools: () => void
  openCommands: () => void
}) {
  const { t } = useI18n()
  const shortcuts = useShortcuts()
  const reconnecting = w.project?.connection === 'reconnecting'
  const ready = w.info.runtime === 'ready' && !reconnecting
  const status = reconnecting
    ? t('Переподключение')
    : w.info.runtime === 'ready'
      ? t('Готов к работе')
      : t('Запускаем Prime')
  return (
    <header className="workspace-topbar">
      <button
        className="icon"
        title={sidebarVisible ? t('Скрыть проекты') : t('Показать проекты')}
        aria-label={t('Панель проектов')}
        data-projects-toggle
        onClick={toggleSidebar}
      >
        {sidebarVisible ? <PanelLeftClose size={17} /> : <PanelLeftOpen size={17} />}
      </button>
      <div className="breadcrumbs" title={status}>
        <span className={`dot ${ready ? 'green' : 'amber'}`} aria-hidden="true" />
        <span className="breadcrumb-project">{projectName || t('Рабочее пространство')}</span>
        <ChevronRight size={13} />
        <strong>{w.session?.title || t('Новый чат')}</strong>
        {!ready && <span className="topbar-status amber-text">{status}</span>}
      </div>
      <button
        className="topbar-search"
        title={`${t('Быстрые действия')} · ${shortcutLabel(shortcuts.commands)}`}
        aria-label={t('Быстрые действия')}
        onClick={openCommands}
      >
        <Search size={15} />
        <span>{t('Поиск и команды')}</span>
        <kbd>{shortcutLabel(shortcuts.commands)}</kbd>
      </button>
      <div className="header-actions">
        {w.messages.length > 0 && (
          <button
            className={`icon ${searchOpen ? 'active-control' : ''}`}
            title={`${t('Поиск в чате')} · ${shortcutLabel(shortcuts.chatSearch)}`}
            aria-label={t('Поиск в чате')}
            aria-expanded={searchOpen}
            onClick={toggleSearch}
          >
            <TextSearch size={17} />
          </button>
        )}
        <ChatActions workspace={w} focusMode={focusMode} toggleFocus={toggleFocus} />
        <span className="topbar-divider" />
        <button
          className={`icon ${toolsOpen ? 'active-control' : ''}`}
          title={`${t('Терминал проекта')} · ${shortcutLabel(shortcuts.terminal)}`}
          aria-label={t('Терминал проекта')}
          aria-expanded={toolsOpen}
          disabled={!w.project}
          onClick={toggleTools}
        >
          <TerminalSquare size={17} />
        </button>
        <button
          className={`icon ${inspectorVisible ? 'active-control' : ''}`}
          title={t('Рабочая область')}
          aria-label={inspectorVisible ? t('Скрыть рабочую область') : t('Показать рабочую область')}
          aria-pressed={inspectorVisible}
          data-inspector-toggle
          onClick={toggleInspector}
        >
          {inspectorVisible ? <PanelRightClose size={17} /> : <PanelRightOpen size={17} />}
        </button>
      </div>
    </header>
  )
}

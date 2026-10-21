import {
  Clock3,
  Command,
  Download,
  Focus,
  PanelLeftOpen,
  PanelRightOpen,
  Plus,
  Puzzle,
  Search,
  Server,
  Settings2,
  SunMoon,
  TerminalSquare,
  Users
} from 'lucide-react'
import { useI18n } from '../i18n'
import type { QuickAction } from '../features/navigation/CommandPalette'
import type { WorkbenchView } from '../features/workbench/WorkbenchDock'
import { downloadConversation } from '../features/chat/exportConversation'
import type { Workspace } from './useWorkspace'

export function useQuickActions({
  workspace: w,
  openTools,
  setModal,
  openSearch,
  focusMode,
  toggleFocus,
  sidebarVisible,
  toggleSidebar,
  inspectorVisible,
  toggleInspector
}: {
  workspace: Workspace
  openTools: (view: WorkbenchView) => void
  setModal: (modal: string) => void
  openSearch: () => void
  focusMode: boolean
  toggleFocus: () => void
  sidebarVisible: boolean
  toggleSidebar: () => void
  inspectorVisible: boolean
  toggleInspector: () => void
}): QuickAction[] {
  const { t } = useI18n()
  return [
    {
      id: 'terminal',
      label: t('Терминал проекта'),
      icon: TerminalSquare,
      shortcut: 'Ctrl `',
      disabled: !w.project,
      run: () => openTools('terminal')
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
      run: () => openTools('teams')
    },
    {
      id: 'automations',
      label: t('Автоматизации'),
      icon: Clock3,
      disabled: !w.project,
      run: () => openTools('automations')
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
      run: openSearch
    },
    {
      id: 'focus',
      label: focusMode ? t('Выйти из фокуса') : t('Режим фокуса'),
      icon: Focus,
      shortcut: 'Ctrl Shift F',
      run: toggleFocus
    },
    {
      id: 'sidebar',
      label: sidebarVisible ? t('Скрыть проекты') : t('Показать проекты'),
      icon: PanelLeftOpen,
      run: toggleSidebar
    },
    {
      id: 'inspector',
      label: inspectorVisible ? t('Скрыть рабочую область') : t('Показать рабочую область'),
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
      run: () => downloadConversation(w.session?.title || t('Новый чат'), w.messages, t)
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
}

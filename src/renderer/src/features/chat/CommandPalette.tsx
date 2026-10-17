import { useEffect, useState } from 'react'
import {
  FolderOpen,
  Plus,
  Search,
  Sparkles,
  Settings,
  PanelRight,
  Puzzle,
  ListRestart,
  Terminal
} from 'lucide-react'
import { Modal } from '../../components/Modal'
import type { Workspace } from '../../app/useWorkspace'
export function CommandPalette({
  workspace: w,
  close,
  settings,
  models,
  plugins,
  inspector
}: {
  workspace: Workspace
  close: () => void
  settings: () => void
  models: () => void
  plugins: () => void
  inspector: () => void
}) {
  const [query, setQuery] = useState('')
  const [custom, setCustom] = useState<any[]>([])
  useEffect(() => {
    if (w.project)
      void Promise.all([w.call('commands').catch(() => []), w.call('mcpPrompts').catch(() => [])])
        .then((value) => setCustom(value.flat()))
        .catch(() => {})
  }, [w.project?.id])
  const actions = [
    { name: 'Новый чат', key: 'Ctrl N', icon: Plus, action: () => void w.createSession() },
    { name: 'Открыть проект', key: '', icon: FolderOpen, action: () => void w.openProject() },
    { name: 'Выбрать модель', key: 'Ctrl L', icon: Sparkles, action: models },
    { name: 'Настройки', key: '', icon: Settings, action: settings },
    { name: 'Плагины', key: '', icon: Puzzle, action: plugins },
    { name: 'Рабочая область', key: '', icon: PanelRight, action: inspector },
    {
      name: 'Сжать контекст',
      key: '',
      icon: ListRestart,
      action: () => void w.call('summarize', { sessionId: w.session?.id }).catch(w.errorHandler)
    },
    ...custom.map((command) => ({
      name: command.Name || command.name || command.title || command.id,
      key: 'Команда проекта',
      icon: Terminal,
      action: () => window.dispatchEvent(new CustomEvent('prime:command', { detail: command }))
    }))
  ].filter((action) => action.name.toLowerCase().includes(query.toLowerCase()))
  return (
    <Modal title="Поиск и команды" close={close}>
      <label className="search-field">
        <Search size={17} />
        <input
          autoFocus
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Введите название действия…"
          onKeyDown={(event) => {
            if (event.key === 'Enter' && actions[0]) {
              close()
              actions[0].action()
            }
          }}
        />
      </label>
      <div className="palette-list">
        {actions.map((action) => (
          <button
            key={action.name}
            onClick={() => {
              close()
              action.action()
            }}
          >
            <action.icon size={16} />
            <span>{action.name}</span>
            <kbd>{action.key}</kbd>
          </button>
        ))}
        {w.sessions
          .filter((session) => session.title.toLowerCase().includes(query.toLowerCase()))
          .slice(0, 8)
          .map((session) => (
            <button
              key={session.id}
              onClick={() => {
                close()
                void w.selectSession(session)
              }}
            >
              <Search size={15} />
              <span>{session.title}</span>
              <small>Чат</small>
            </button>
          ))}
      </div>
    </Modal>
  )
}

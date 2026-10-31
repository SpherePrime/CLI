import { GitHistoryPanel } from '../github/GitHistoryPanel'
import { projectName, projectRoot } from '../projects/projectName'
import { Select } from '../../components/Select'
import { useCallback, useEffect, useRef, useState, type MouseEvent } from 'react'
import {
  ChevronDown,
  ChevronRight,
  Code2,
  Copy,
  ExternalLink,
  File,
  FilePlus,
  Folder,
  FolderOpen,
  FolderPlus,
  RefreshCw,
  ArrowUpRight,
  FileDiff,
  Files,
  Trash2,
  Globe,
  Users
} from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'
import { FileContextMenu, type MenuItem } from './FileContextMenu'
import { EntryDeleteDialog, EntryNameDialog } from './EntryDialogs'
import { BrowserTab } from '../browser/BrowserTab'
import { notifyError } from '../notifications/notifications'
import { AgentsPanel } from '../agents/AgentsPanel'
import type { AgentRequest } from '../agents/openAgent'
import { realSessionId } from '../../../../shared/pendingSession'

type Entry = { path: string; name: string; directory: boolean }

function parentFolder(entry: Entry | null) {
  if (!entry) return ''
  if (entry.directory) return entry.path
  const index = Math.max(entry.path.lastIndexOf('/'), entry.path.lastIndexOf('\\'))
  return index > 0 ? entry.path.slice(0, index) : ''
}

export function Inspector({
  workspace: w,
  openDocument,
  compact = false,
  expanded = false,
  setExpanded,
  browserRequest = 0,
  agentRequest
}: {
  workspace: Workspace
  openDocument: (workspaceId: string, path: string, diff?: any) => Promise<void>
  compact?: boolean
  expanded?: boolean
  setExpanded: (expanded: boolean) => void
  browserRequest?: number
  agentRequest?: AgentRequest
}) {
  const { t } = useI18n()
  const [tab, setTab] = useState('files')
  const [panelOpen, setPanelOpen] = useState(false)
  const panel = useRef<HTMLElement>(null)
  useEffect(() => {
    const toggle = () => setPanelOpen((value) => !value)
    window.addEventListener('prime:inspector-panel', toggle)
    return () => window.removeEventListener('prime:inspector-panel', toggle)
  }, [])
  useEffect(() => {
    if (!compact || !panelOpen) return
    const pointer = (event: PointerEvent) => {
      const target = event.target as Element
      if (!panel.current?.contains(target) && !target.closest?.('[data-inspector-toggle]'))
        setPanelOpen(false)
    }
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setPanelOpen(false)
        panel.current?.querySelector<HTMLButtonElement>('.inspector-tabs .active')?.focus()
      }
    }
    document.addEventListener('pointerdown', pointer)
    document.addEventListener('keydown', keyboard)
    return () => {
      document.removeEventListener('pointerdown', pointer)
      document.removeEventListener('keydown', keyboard)
    }
  }, [compact, panelOpen])
  const [history, setHistory] = useState<any[]>([])
  const [changes, setChanges] = useState<{ path: string; status: string }[]>([])
  const [gitError, setGitError] = useState('')
  const [source, setSource] = useState<'session' | 'git'>('session')
  const [error, setError] = useState('')
  useEffect(() => {
    if (!error) return
    notifyError(error)
    setError('')
  }, [error])
  const [treeKey, setTreeKey] = useState(0)
  const [opened, setOpened] = useState<string[]>([])
  const [menu, setMenu] = useState<{ x: number; y: number; entry: Entry | null } | null>(null)
  const [creating, setCreating] = useState<{ kind: 'file' | 'folder'; parent: string } | null>(null)
  const [removing, setRemoving] = useState<Entry | null>(null)
  const closeMenu = useCallback(() => setMenu(null), [])
  useEffect(() => {
    if (browserRequest) {
      setTab('browser')
      if (compact) setPanelOpen(true)
    }
  }, [browserRequest])
  useEffect(() => {
    if (!agentRequest) return
    setTab('agents')
    if (compact) setPanelOpen(true)
    else setExpanded(true)
  }, [agentRequest?.sequence])
  useEffect(() => setOpened([]), [w.project?.id])
  useEffect(() => {
    let current = true
    setChanges([])
    setGitError('')
    if (w.project && tab === 'changes' && source === 'git') {
      void window.prime
        .changes(w.project.id)
        .then((items) => {
          if (current) setChanges(items)
        })
        .catch((error) => {
          if (current) setGitError(error.message)
        })
    }
    return () => {
      current = false
    }
  }, [w.project?.id, w.busy, tab, source, treeKey])
  useEffect(() => {
    setTreeKey((key) => key + 1)
  }, [w.busy])
  useEffect(() => {
    const refresh = () => setTreeKey((key) => key + 1)
    window.addEventListener('prime:file-saved', refresh)
    return () => window.removeEventListener('prime:file-saved', refresh)
  }, [])
  useEffect(() => {
    setHistory([])
    const sessionId = realSessionId(w.session)
    if (!w.project || !sessionId) return
    void w
      .call('history', { sessionId })
      .then((items) => setHistory(items || []))
      .catch(() => {})
  }, [w.project?.id, w.session?.id, w.busy])
  async function openFile(path: string) {
    try {
      await openDocument(w.project!.id, path)
      setError('')
    } catch (error: any) {
      setError(error.message)
    }
  }
  const toggleFolder = (path: string) =>
    setOpened((items) =>
      items.includes(path) ? items.filter((item) => item !== path) : [...items, path]
    )
  const showMenu = (event: MouseEvent, entry: Entry | null) => {
    event.preventDefault()
    event.stopPropagation()
    setMenu({ x: event.clientX, y: event.clientY, entry })
  }
  const attempt = (action: () => Promise<unknown>) =>
    void action().catch((reason) => setError(reason?.message || String(reason)))
  function menuItems(entry: Entry | null): MenuItem[] {
    const id = w.project!.id
    const target = entry?.path || ''
    const parent = parentFolder(entry)
    const remote = w.project!.remote
    const absolute = remote
      ? [remote.root.replace(/\/$/, ''), target].filter(Boolean).join('/')
      : [w.project!.path, target.replaceAll('/', '\\')].filter(Boolean).join('\\')
    const items: MenuItem[] = []
    if (entry && !entry.directory)
      items.push({
        label: t('Открыть'),
        icon: <File size={14} />,
        action: () => void openFile(entry.path)
      })
    items.push({
      label: t('Открыть в VS Code'),
      icon: <Code2 size={14} />,
      action: () => attempt(() => window.prime.openWith(id, target, 'vscode'))
    })
    if (entry && !entry.directory && !remote)
      items.push({
        label: t('Открыть программой по умолчанию'),
        icon: <ExternalLink size={14} />,
        action: () => attempt(() => window.prime.openWith(id, target, 'default'))
      })
    if (!remote)
      items.push({
        label: t('Показать в проводнике'),
        icon: <FolderOpen size={14} />,
        action: () => attempt(() => window.prime.reveal(id, target))
      })
    items.push(
      {
        label: t('Копировать путь'),
        icon: <Copy size={14} />,
        action: () => attempt(() => navigator.clipboard.writeText(absolute))
      },
      { separator: true },
      {
        label: t('Новый файл'),
        icon: <FilePlus size={14} />,
        action: () => setCreating({ kind: 'file', parent })
      },
      {
        label: t('Новая папка'),
        icon: <FolderPlus size={14} />,
        action: () => setCreating({ kind: 'folder', parent })
      }
    )
    if (entry)
      items.push(
        { separator: true },
        {
          label: t('Удалить'),
          icon: <Trash2 size={14} />,
          danger: true,
          action: () => setRemoving(entry)
        }
      )
    return items
  }
  async function diffFile(path: string) {
    try {
      const root = projectRoot(w.project!)
      const relative = path.startsWith(root) ? path.slice(root.length).replace(/^[\\/]/, '') : path
      await openDocument(
        w.project!.id,
        relative,
        await window.prime.diff(w.project!.id, realSessionId(w.session) || '', relative, source)
      )
      setError('')
    } catch (error: any) {
      setError(error.message)
    }
  }
  const open = compact ? panelOpen : expanded
  function select(id: string) {
    if (open && tab === id) return compact ? setPanelOpen(false) : setExpanded(false)
    setTab(id)
    if (compact) setPanelOpen(true)
    else setExpanded(true)
  }
  const body = (
    <>
      {tab === 'browser' && (
        <div className="inspector-browser">
          <BrowserTab
            workspace={w}
            onSend={(prompt, attachmentIds) => w.send(prompt, attachmentIds)}
          />
        </div>
      )}
      <div
        className={`inspector-content ${tab === 'agents' ? 'agents-content' : ''}`}
        hidden={tab === 'browser'}
      >
        {tab === 'agents' && <AgentsPanel workspace={w} request={agentRequest} />}
        {tab === 'files' && (
          <>
            <div className="panel-title">
              <span>{projectName(w.project) || t('ПРОЕКТ')}</span>
              <button
                className="icon small"
                title={t('Обновить дерево')}
                aria-label={t('Обновить дерево')}
                onClick={() => setTreeKey((key) => key + 1)}
              >
                <RefreshCw size={13} />
              </button>
            </div>
            {w.project ? (
              <div className="file-tree-area" onContextMenu={(event) => showMenu(event, null)}>
                <Tree
                  key={`${w.project.id}:${treeKey}`}
                  workspaceId={w.project.id}
                  opened={opened}
                  onToggle={toggleFolder}
                  onFile={openFile}
                  onMenu={showMenu}
                  onError={setError}
                />
              </div>
            ) : (
              <div className="panel-empty">
                <FolderOpen size={28} />
                <p>{t('Файлы выбранного проекта')}</p>
              </div>
            )}
          </>
        )}
        {tab === 'changes' && (
          <>
            {w.project && (
              <details className="inspector-history">
                <summary>{t('История коммитов')}</summary>
                <GitHistoryPanel key={w.project.id} workspaceId={w.project.id} />
              </details>
            )}
            <p className="panel-help">
              {source === 'git'
                ? t('Все изменения проекта относительно Git HEAD.')
                : t('Изменения файлов в истории текущего чата. Файлы уже записаны на диск.')}
            </p>
            <Select
              label={t('Источник сравнения')}
              value={source}
              onChange={(value) => setSource(value as 'session' | 'git')}
              options={[
                { value: 'session', label: t('История сессии') },
                { value: 'git', label: t('Сравнить с Git HEAD') }
              ]}
            />
            <button className="quiet" onClick={() => setTreeKey((key) => key + 1)}>
              <RefreshCw size={13} />
              {t('Обновить')}
            </button>
            {gitError && source === 'git' && <p className="error-text">{gitError}</p>}
            {(source === 'git'
              ? changes.map((item) => item.path)
              : [...new Set(history.map((item) => item.path))]
            ).map((path) => (
              <button className="tree-file" key={path} onClick={() => void diffFile(path)}>
                <FileDiff size={14} />
                <span title={path}>{path}</span>
                {source === 'git' && (
                  <small>{changes.find((item) => item.path === path)?.status}</small>
                )}
              </button>
            ))}
            {!(source === 'git' ? changes : history).length && !gitError && (
              <div className="panel-empty">
                <FileDiff size={28} />
                <p>{t('Изменения появятся после редактирования файлов')}</p>
              </div>
            )}
          </>
        )}
      </div>
      {menu && w.project && (
        <FileContextMenu x={menu.x} y={menu.y} items={menuItems(menu.entry)} close={closeMenu} />
      )}
      {creating && w.project && (
        <EntryNameDialog
          kind={creating.kind}
          parent={creating.parent}
          close={() => setCreating(null)}
          create={async (path) => {
            await window.prime.createEntry(w.project!.id, path, creating.kind)
            const parent = creating.parent
            if (parent) setOpened((items) => (items.includes(parent) ? items : [...items, parent]))
            setTreeKey((key) => key + 1)
            if (creating.kind === 'file') void openFile(path)
          }}
        />
      )}
      {removing && w.project && (
        <EntryDeleteDialog
          path={removing.path}
          directory={removing.directory}
          permanent={Boolean(w.project.remote)}
          close={() => setRemoving(null)}
          remove={async () => {
            await window.prime.trashEntry(w.project!.id, removing.path)
            setTreeKey((key) => key + 1)
          }}
        />
      )}
      {w.project && !w.project.remote && tab !== 'browser' && tab !== 'agents' && (
        <div className="inspector-footer">
          <button onClick={() => void window.prime.reveal(w.project!.id)}>
            <FolderOpen size={13} /> {t('Открыть в проводнике')}
            <ArrowUpRight size={12} />
          </button>
        </div>
      )}
    </>
  )
  return (
    <>
      {!compact && expanded && <aside className="inspector inspector-panel">{body}</aside>}
      <aside
        ref={compact ? panel : undefined}
        className={`inspector inspector-rail ${compact && panelOpen ? 'panel-open' : ''}`}
      >
        <div className="inspector-tabs">
          {[
            { id: 'files', icon: Files, title: t('Файлы') },
            { id: 'changes', icon: FileDiff, title: t('Изменения') },
            { id: 'browser', icon: Globe, title: t('Браузер') },
            { id: 'agents', icon: Users, title: t('Агенты') }
          ].map((item) => (
            <button
              key={item.id}
              title={item.title}
              aria-label={item.title}
              className={open && tab === item.id ? 'active' : ''}
              aria-pressed={open && tab === item.id}
              onClick={() => select(item.id)}
            >
              <item.icon size={15} />
              <span>{item.title}</span>
            </button>
          ))}
        </div>
        {compact && panelOpen && body}
      </aside>
    </>
  )
}
function Tree({
  workspaceId,
  path = '',
  opened,
  onToggle,
  onFile,
  onMenu,
  onError
}: {
  workspaceId: string
  path?: string
  opened: string[]
  onToggle: (path: string) => void
  onFile: (path: string) => void
  onMenu: (event: MouseEvent, entry: Entry) => void
  onError: (error: string) => void
}) {
  const [entries, setEntries] = useState<Entry[]>([])
  useEffect(() => {
    void window.prime
      .files(workspaceId, path)
      .then((items) => setEntries(items as Entry[]))
      .catch((error) => onError(error.message))
  }, [workspaceId, path])
  return (
    <div className="file-tree">
      {entries.map((entry) => (
        <div key={entry.path}>
          <button
            className="tree-file"
            onClick={() => (entry.directory ? onToggle(entry.path) : onFile(entry.path))}
            onContextMenu={(event) => onMenu(event, entry)}
          >
            {entry.directory ? (
              <>
                {opened.includes(entry.path) ? (
                  <ChevronDown size={12} />
                ) : (
                  <ChevronRight size={12} />
                )}
                <Folder size={14} />
              </>
            ) : (
              <>
                <span className="tree-spacer" />
                <File size={14} />
              </>
            )}
            <span>{entry.name}</span>
          </button>
          {entry.directory && opened.includes(entry.path) && (
            <div className="tree-children">
              <Tree
                workspaceId={workspaceId}
                path={entry.path}
                opened={opened}
                onToggle={onToggle}
                onFile={onFile}
                onMenu={onMenu}
                onError={onError}
              />
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

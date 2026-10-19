import { projectName, projectRoot } from '../projects/projectName'
import { Select } from '../../components/Select'
import { useCallback, useEffect, useState, type MouseEvent } from 'react'
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
  PanelRightClose,
  ArrowUpRight,
  FileDiff,
  Files,
  ListTodo,
  Play,
  Trash2,
  Globe
} from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'
import { FileContextMenu, type MenuItem } from './FileContextMenu'
import { EntryDeleteDialog, EntryNameDialog } from './EntryDialogs'
import { BrowserTab } from '../browser/BrowserTab'

type Entry = { path: string; name: string; directory: boolean }

function parentFolder(entry: Entry | null) {
  if (!entry) return ''
  if (entry.directory) return entry.path
  const index = Math.max(entry.path.lastIndexOf('/'), entry.path.lastIndexOf('\\'))
  return index > 0 ? entry.path.slice(0, index) : ''
}

export function Inspector({
  workspace: w,
  close,
  openDocument,
  browserRequest = 0
}: {
  workspace: Workspace
  close: () => void
  openDocument: (workspaceId: string, path: string, diff?: any) => Promise<void>
  browserRequest?: number
}) {
  const { t } = useI18n()
  const [tab, setTab] = useState('files')
  const [history, setHistory] = useState<any[]>([])
  const [changes, setChanges] = useState<{ path: string; status: string }[]>([])
  const [gitError, setGitError] = useState('')
  const [jobs, setJobs] = useState<any[]>([])
  const [source, setSource] = useState<'session' | 'git'>('session')
  const [error, setError] = useState('')
  const [treeKey, setTreeKey] = useState(0)
  const [opened, setOpened] = useState<string[]>([])
  const [menu, setMenu] = useState<{ x: number; y: number; entry: Entry | null } | null>(null)
  const [creating, setCreating] = useState<{ kind: 'file' | 'folder'; parent: string } | null>(null)
  const [removing, setRemoving] = useState<Entry | null>(null)
  const closeMenu = useCallback(() => setMenu(null), [])
  useEffect(() => {
    if (browserRequest) setTab('browser')
  }, [browserRequest])
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
    setJobs([])
    if (!w.project || !w.session) return
    void w
      .call('history', { sessionId: w.session.id })
      .then((items) => setHistory(items || []))
      .catch(() => {})
    void w
      .call('jobs', { sessionId: w.session.id })
      .then((items) => setJobs(items || []))
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
        await window.prime.diff(w.project!.id, w.session?.id || '', relative, source)
      )
      setError('')
    } catch (error: any) {
      setError(error.message)
    }
  }
  return (
    <aside className="inspector">
      <div className="inspector-header">
        <strong>{t('Рабочая область')}</strong>
        <button
          className="icon"
          title={t('Скрыть панель')}
          aria-label={t('Скрыть панель')}
          onClick={close}
        >
          <PanelRightClose size={16} />
        </button>
      </div>
      <div className="inspector-tabs">
        {[
          { id: 'files', icon: Files, title: t('Файлы') },
          { id: 'changes', icon: FileDiff, title: t('Изменения') },
          { id: 'tasks', icon: ListTodo, title: t('Задачи') },
          { id: 'browser', icon: Globe, title: t('Браузер') }
        ].map((item) => (
          <button
            key={item.id}
            title={item.title}
            aria-label={item.title}
            className={tab === item.id ? 'active' : ''}
            onClick={() => setTab(item.id)}
          >
            <item.icon size={15} />
            <span>{item.title}</span>
          </button>
        ))}
      </div>
      {tab === 'browser' && (
        <BrowserTab onSend={(prompt, attachmentIds) => w.send(prompt, attachmentIds)} />
      )}
      <div className="inspector-content" hidden={tab === 'browser'}>
        {error && (
          <div className="panel-error">
            {error}
            <button className="quiet" onClick={() => setError('')}>
              {t('Закрыть')}
            </button>
          </div>
        )}
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
        {tab === 'tasks' && (
          <>
            <h4>{t('План задачи')}</h4>
            {w.session?.todos?.map((todo: any, index) => (
              <div className={`todo-item ${todo.status}`} key={index}>
                <span>
                  {todo.status === 'completed' ? '✓' : todo.status === 'in_progress' ? '◉' : '○'}
                </span>
                {todo.content}
              </div>
            ))}
            {!w.session?.todos?.length && (
              <p className="panel-help">
                {t('Агент покажет здесь план, когда создаст список задач.')}
              </p>
            )}
            <h4>{t('Фоновые команды')}</h4>
            {jobs.map((job) => (
              <details className="job" key={job.id}>
                <summary>
                  <span className={`dot ${job.done ? 'green' : 'amber'}`} />
                  {job.description || job.command}
                </summary>
                <pre>
                  {job.stdout}
                  {job.stderr}
                  {job.error}
                </pre>
              </details>
            ))}
            {!jobs.length && <p className="panel-help">{t('Нет фоновых команд в этой сессии')}</p>}
            {w.sessions
              .filter((item) => item.parent_session_id === w.session?.id)
              .map((item) => (
                <button
                  className="tree-file"
                  key={item.id}
                  onClick={() => void w.selectSession(item)}
                >
                  <Play size={13} />
                  {item.title}
                </button>
              ))}
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
      {w.project && !w.project.remote && tab !== 'browser' && (
        <div className="inspector-footer">
          <button onClick={() => void window.prime.reveal(w.project!.id)}>
            <FolderOpen size={13} /> {t('Открыть в проводнике')}
            <ArrowUpRight size={12} />
          </button>
        </div>
      )}
    </aside>
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

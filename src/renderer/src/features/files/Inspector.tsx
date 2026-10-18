import { Select } from '../../components/Select'
import { useEffect, useState } from 'react'
import {
  ChevronDown,
  ChevronRight,
  File,
  Folder,
  FolderOpen,
  RefreshCw,
  PanelRightClose,
  ArrowUpRight,
  FileDiff,
  Files,
  ListTodo,
  Play
} from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'

export function Inspector({
  workspace: w,
  close,
  openDocument
}: {
  workspace: Workspace
  close: () => void
  openDocument: (workspaceId: string, path: string, diff?: any) => Promise<void>
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
  async function diffFile(path: string) {
    try {
      const relative = path.startsWith(w.project!.path)
        ? path.slice(w.project!.path.length).replace(/^[\\/]/, '')
        : path
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
          { id: 'tasks', icon: ListTodo, title: t('Задачи') }
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
      <div className="inspector-content">
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
              <span>{w.project?.path.split(/[\\/]/).pop() || t('ПРОЕКТ')}</span>
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
              <Tree
                key={`${w.project.id}:${treeKey}`}
                workspaceId={w.project.id}
                onFile={openFile}
                onError={setError}
              />
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
      {w.project && (
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
  onFile,
  onError
}: {
  workspaceId: string
  path?: string
  onFile: (path: string) => void
  onError: (error: string) => void
}) {
  const [entries, setEntries] = useState<any[]>([])
  const [opened, setOpened] = useState<string[]>([])
  useEffect(() => {
    void window.prime
      .files(workspaceId, path)
      .then(setEntries)
      .catch((error) => onError(error.message))
  }, [workspaceId, path])
  return (
    <div className="file-tree">
      {entries.map((entry) => (
        <div key={entry.path}>
          <button
            className="tree-file"
            onClick={() =>
              entry.directory
                ? setOpened((items) =>
                    items.includes(entry.path)
                      ? items.filter((item) => item !== entry.path)
                      : [...items, entry.path]
                  )
                : onFile(entry.path)
            }
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
              <Tree workspaceId={workspaceId} path={entry.path} onFile={onFile} onError={onError} />
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

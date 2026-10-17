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
  Boxes,
  ListTodo,
  Play,
  Shield,
  RotateCw
} from 'lucide-react'
import { Modal } from '../../components/Modal'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'

export function Inspector({ workspace: w, close }: { workspace: Workspace; close: () => void }) {
  const { t } = useI18n()
  const [tab, setTab] = useState('files')
  const [preview, setPreview] = useState<any>(null)
  const [detail, setDetail] = useState<any>(null)
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
      setPreview(await window.prime.readFile(w.project!.id, path))
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
      setPreview(await window.prime.diff(w.project!.id, w.session?.id || '', relative, source))
      setError('')
    } catch (error: any) {
      setError(error.message)
    }
  }
  const resources: { label: string; key: string; entries: any[] }[] = [
    {
      label: 'MCP',
      key: 'mcp',
      entries: Object.entries(w.resources.mcp || {}).map(([name, item]: [string, any]) => ({
        ...item,
        name
      }))
    },
    {
      label: 'LSP',
      key: 'lsp',
      entries: Object.entries(w.resources.lsp || {}).map(([name, item]: [string, any]) => ({
        ...item,
        name
      }))
    },
    {
      label: 'Skills',
      key: 'skills',
      entries: Array.isArray(w.resources.skills) ? w.resources.skills : []
    }
  ]
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
          { id: 'resources', icon: Boxes, title: t('Ресурсы') },
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
            <select
              aria-label={t('Источник сравнения')}
              value={source}
              onChange={(event) => setSource(event.target.value as 'session' | 'git')}
            >
              <option value="session">{t('История сессии')}</option>
              <option value="git">{t('Сравнить с Git HEAD')}</option>
            </select>
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
        {tab === 'resources' && (
          <>
            <button
              className="quiet full-width"
              disabled={!w.project}
              onClick={() => void w.call('restart').then(w.refresh).catch(w.errorHandler)}
            >
              <RotateCw size={14} /> {t('Перезапустить системы')}
            </button>
            <div className="button-row">
              <button
                className="quiet"
                disabled={!w.project}
                onClick={() => void w.call('dockerEnable').then(w.refresh).catch(w.errorHandler)}
              >
                {t('Docker MCP: вкл.')}
              </button>
              <button
                className="quiet"
                disabled={!w.project}
                onClick={() => void w.call('dockerDisable').then(w.refresh).catch(w.errorHandler)}
              >
                {t('выкл.')}
              </button>
            </div>
            {resources.map((group) => (
              <section className="resource-group" key={group.key}>
                <h4>
                  {group.label}
                  <span>{group.entries.length}</span>
                </h4>
                {group.entries.map((item: any, index) => (
                  <button
                    key={item.id || item.name || index}
                    className="resource-item"
                    onClick={() => setDetail({ ...item, kind: group.key })}
                  >
                    <span
                      className={`dot ${item.error ? 'red' : item.state === 'connected' || group.key === 'skills' ? 'green' : 'amber'}`}
                    />
                    <span>
                      <strong>{item.name || item.label || item.id || item.Name}</strong>
                      <small>
                        {item.error
                          ? t('Ошибка')
                          : item.state ||
                            item.source ||
                            item.description?.slice(0, 70) ||
                            t('Доступно')}
                      </small>
                    </span>
                    <ChevronRight size={13} />
                  </button>
                ))}
                {!group.entries.length && <p className="panel-help">{t('Не настроено')}</p>}
              </section>
            ))}
            <section className="resource-group">
              <h4>
                {t('MCP-каналы')}
                <span>{w.resources.channels?.length || 0}</span>
              </h4>
              {(w.resources.channels || []).map((message: any) => (
                <details key={message.id} className="job">
                  <summary>
                    {message.name} · {new Date(message.received_at).toLocaleTimeString()}
                  </summary>
                  <pre>{message.content}</pre>
                  <button
                    className="quiet"
                    onClick={() =>
                      window.dispatchEvent(
                        new CustomEvent('prime:draft', { detail: message.content })
                      )
                    }
                  >
                    {t('Подготовить запрос в чате')}
                  </button>
                </details>
              ))}
              {!w.resources.channels?.length && (
                <p className="panel-help">
                  {t('Каналы включаются для проекта в настройках Prime.')}
                </p>
              )}
            </section>
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
      {preview && (
        <Modal title={preview.name || preview.path} close={() => setPreview(null)} wide>
          {preview.image ? (
            <img className="file-image" src={preview.image} alt={preview.name} />
          ) : (
            <pre className={`file-preview ${preview.patch ? 'diff-preview' : ''}`}>
              {preview.patch
                ? preview.patch.split('\n').map((line: string, index: number) => (
                    <div
                      key={index}
                      className={
                        line.startsWith('+')
                          ? 'addition'
                          : line.startsWith('-')
                            ? 'deletion'
                            : line.startsWith('@@')
                              ? 'hunk'
                              : ''
                      }
                    >
                      {line || ' '}
                    </div>
                  ))
                : preview.content}
            </pre>
          )}
          <button
            className="quiet"
            onClick={() => void window.prime.reveal(w.project!.id, preview.path)}
          >
            {t('Показать файл в проводнике')}
          </button>
        </Modal>
      )}
      {detail && <ResourceDetails workspace={w} detail={detail} close={() => setDetail(null)} />}
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
function ResourceDetails({
  workspace: w,
  detail,
  close
}: {
  workspace: Workspace
  detail: any
  close: () => void
}) {
  const { t } = useI18n()
  const [output, setOutput] = useState('')
  const [status, setStatus] = useState('')
  const [uri, setUri] = useState('')
  async function run(operation: any, args: any) {
    try {
      setOutput(JSON.stringify(await w.call(operation, args), null, 2))
      await w.refresh()
    } catch (error: any) {
      setStatus(error.message)
    }
  }
  return (
    <Modal
      title={`${detail.kind.toUpperCase()} · ${detail.name || detail.id || detail.Name}`}
      close={close}
    >
      <pre className="config-preview">{JSON.stringify(detail, null, 2)}</pre>
      <div className="button-row">
        {detail.kind === 'mcp' && (
          <>
            <button onClick={() => void run('mcpRefresh', { name: detail.name })}>
              {t('Обновить инструменты')}
            </button>
            <button
              onClick={async () => {
                const auth = w.call('mcpAuth', { name: detail.name })
                const timer = setInterval(() => {
                  void w
                    .call('mcpAuthUrl', { name: detail.name })
                    .then((value) => {
                      if (value.auth_url) {
                        clearInterval(timer)
                        return window.prime.openExternal(value.auth_url)
                      }
                    })
                    .catch(() => clearInterval(timer))
                }, 600)
                try {
                  await auth
                  setStatus(t('Авторизация завершена'))
                } catch (error: any) {
                  setStatus(error.message)
                } finally {
                  clearInterval(timer)
                }
              }}
            >
              {t('Авторизация')}
            </button>
            <label className="form-label">
              {t('URI ресурса')}
              <input value={uri} onChange={(event) => setUri(event.target.value)} />
            </label>
            <button
              disabled={!uri}
              onClick={() => void run('mcpResource', { name: detail.name, uri })}
            >
              {t('Прочитать ресурс')}
            </button>
            <button onClick={() => void run('mcpPrompts', {})}>{t('Шаблоны запросов')}</button>
          </>
        )}
        {detail.kind === 'lsp' && (
          <>
            <button onClick={() => void run('diagnostics', { name: detail.name })}>
              {t('Диагностика')}
            </button>
            <button onClick={() => void run('lspStart', { path: w.project?.path })}>
              {t('Запустить')}
            </button>
            <button onClick={() => void run('lspStop', {})}>{t('Остановить LSP')}</button>
          </>
        )}
        {detail.kind === 'skills' && (
          <button
            onClick={() =>
              void w
                .call('readSkill', { id: detail.id || detail.name })
                .then((result) =>
                  setOutput(result.content ? atob(result.content) : JSON.stringify(result, null, 2))
                )
                .catch((error) => setStatus(error.message))
            }
          >
            {t('Открыть инструкции')}
          </button>
        )}
      </div>
      {status && <p className="error-text">{status}</p>}
      {output && <pre className="config-preview">{output}</pre>}
    </Modal>
  )
}

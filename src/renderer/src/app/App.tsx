import { useDocuments } from '../features/files/useDocuments'
import { StartupScreen } from '../components/StartupScreen'
import { UpdateScreen } from '../features/updates/UpdateScreen'
import { Titlebar } from '../components/Titlebar'
import { I18nProvider, useI18n } from '../i18n'
import { PrimeLogo } from '../components/PrimeLogo'
import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import {
  ArrowUpRight,
  Code2,
  FileSearch,
  FolderOpen,
  GitBranch,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightOpen,
  Search,
  Sparkles,
  Terminal,
  X,
  AlertCircle,
  LoaderCircle,
  ChevronRight
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
        busy={w.busy}
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
  const [widths, setWidths] = useState({ sidebar: 256, inspector: 334 })
  const [zoom, setZoom] = useState(1)
  const shell = useRef<HTMLDivElement>(null)
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
  }, [w.project?.id])
  function resize(which: 'sidebar' | 'inspector', event: React.PointerEvent<HTMLDivElement>) {
    event.preventDefault()
    event.currentTarget.setPointerCapture(event.pointerId)
    const start = event.clientX
    const initial = widths[which]
    const move = (move: PointerEvent) =>
      setWidths((value) => ({
        ...value,
        [which]: Math.min(
          which === 'sidebar' ? 400 : 650,
          Math.max(
            which === 'sidebar' ? 210 : 280,
            initial + (move.clientX - start) * (which === 'sidebar' ? 1 : -1)
          )
        )
      }))
    const stop = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
      setWidths((value) => {
        void w.savePreferences({ sidebarWidth: value.sidebar, inspectorWidth: value.inspector })
        return value
      })
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop)
  }
  const projectName =
    w.project?.kind === 'chat' ? t('Личные чаты') : w.project?.path.split(/[\\/]/).pop()
  const hasFolder = Boolean(w.project && w.project.kind !== 'chat')
  const isEmpty = !w.messages.length && !w.busy
  return (
    <>
      <Titlebar />
      <div
        ref={shell}
        className={`app-shell ${sidebar ? '' : 'hide-sidebar'} ${inspector ? '' : 'hide-inspector'} ${documents.documents.length ? 'has-editor' : ''}`}
        style={
          {
            '--sidebar-width': widths.sidebar + 'px',
            '--editor-fraction': `${editorFraction}fr`,
            '--inspector-width': widths.inspector + 'px',
            zoom
          } as React.CSSProperties
        }
      >
        {sidebar && (
          <>
            <Sidebar
              workspace={w}
              onSettings={() => setModal('settings')}
              onPlugins={() => setModal('plugins')}
            />
            <div
              className="resizer left-resizer"
              role="separator"
              aria-label={t('Ширина панели проектов')}
              tabIndex={0}
              onPointerDown={(event) => resize('sidebar', event)}
              onKeyDown={(event) => {
                if (event.key === 'ArrowLeft' || event.key === 'ArrowRight')
                  setWidths((value) => ({
                    ...value,
                    sidebar: Math.max(
                      210,
                      Math.min(400, value.sidebar + (event.key === 'ArrowRight' ? 10 : -10))
                    )
                  }))
              }}
            />
          </>
        )}
        <main className="main-workspace">
          <header className="workspace-header">
            <button
              className="icon"
              title={sidebar ? t('Скрыть проекты') : t('Показать проекты')}
              aria-label={t('Панель проектов')}
              onClick={() => setSidebar((value) => !value)}
            >
              {sidebar ? <PanelLeftClose size={17} /> : <PanelLeftOpen size={17} />}
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
                    ? t('Локальный runtime')
                    : t('Запускаем Prime')}
              </span>
              <ChatActions workspace={w} />
              {!inspector && (
                <button
                  className="icon"
                  aria-label={t('Показать рабочую область')}
                  title={t('Рабочая область')}
                  onClick={() => setInspector(true)}
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
            <section className="welcome">
              <div className="welcome-wordmark">
                <PrimeLogo />
              </div>
              <div className="eyebrow">
                {hasFolder ? t('ВАШ ПРОЕКТ. ВАШИ ИНСТРУМЕНТЫ.') : t('ВАШИ ИДЕИ. ВАШ PRIME.')}
              </div>
              <h1>{hasFolder ? t('Что сделаем в проекте?') : t('С чего начнём?')}</h1>
              <p>
                {hasFolder ? (
                  <>
                    {t('Prime работает в')} <strong>{projectName}</strong>.<br />
                    {t('Опишите задачу — контекст и инструменты уже рядом.')}
                  </>
                ) : (
                  <>
                    {t('Выберите модель и начните разговор.')}
                    <br />
                    {t('Папку можно подключить, когда понадобятся файлы проекта.')}
                  </>
                )}
              </p>
              {!hasFolder && (
                <button
                  className="primary open-project"
                  disabled={w.loading || w.info.runtime !== 'ready'}
                  onClick={() => void w.openProject()}
                >
                  <FolderOpen size={16} /> {t('Открыть проект')}
                  <ArrowUpRight size={15} />
                </button>
              )}
              <div className="starter-grid">
                {(hasFolder
                  ? [
                      {
                        icon: FileSearch,
                        title: t('Разобраться в проекте'),
                        description: t('Архитектура, файлы и связи'),
                        prompt: t('Изучи проект и объясни его архитектуру и основные точки входа.')
                      },
                      {
                        icon: Code2,
                        title: t('Внести изменения'),
                        description: t('От задачи к готовому решению'),
                        prompt: t('Изучи проект и помоги реализовать следующую задачу: ')
                      },
                      {
                        icon: Terminal,
                        title: t('Найти проблему'),
                        description: t('Ошибки, причины и проверки'),
                        prompt: t(
                          'Проверь проект на ошибки и предложи конкретные исправления с проверками.'
                        )
                      }
                    ]
                  : [
                      {
                        icon: Sparkles,
                        title: t('Обсудить идею'),
                        description: t('Найти направление и решение'),
                        prompt: t('Помоги продумать идею: ')
                      },
                      {
                        icon: FileSearch,
                        title: t('Разобраться в вопросе'),
                        description: t('Понятно объяснить сложное'),
                        prompt: t('Помоги разобраться в следующем вопросе: ')
                      },
                      {
                        icon: Code2,
                        title: t('Составить план'),
                        description: t('От замысла к конкретным шагам'),
                        prompt: t('Помоги составить план для следующей задачи: ')
                      }
                    ]
                ).map((item) => (
                  <button
                    key={item.title}
                    onClick={() =>
                      window.dispatchEvent(new CustomEvent('prime:draft', { detail: item.prompt }))
                    }
                  >
                    <item.icon size={19} />
                    <strong>{item.title}</strong>
                    <span>{item.description}</span>
                    <ArrowUpRight className="starter-arrow" size={13} />
                  </button>
                ))}
              </div>
            </section>
          ) : (
            <MessageList
              key={w.session?.id || 'new'}
              messages={w.messages}
              busy={w.busy}
              waiting={
                w.permissions.some((item) => item.session_id === w.session?.id) ||
                w.questions.some((item) => item.session_id === w.session?.id)
              }
            />
          )}
          <Approvals workspace={w} />
          <Composer workspace={w} />
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
        {inspector && (
          <>
            <div
              className="resizer right-resizer"
              role="separator"
              aria-label={t('Ширина рабочей области')}
              tabIndex={0}
              onPointerDown={(event) => resize('inspector', event)}
              onKeyDown={(event) => {
                if (event.key === 'ArrowLeft' || event.key === 'ArrowRight')
                  setWidths((value) => ({
                    ...value,
                    inspector: Math.max(
                      280,
                      Math.min(650, value.inspector + (event.key === 'ArrowLeft' ? 10 : -10))
                    )
                  }))
              }}
            />
            <Inspector
              workspace={w}
              close={() => setInspector(false)}
              openDocument={documents.open}
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
              <FileEditor state={documents} theme={w.preferences.theme} />
            </Suspense>
          </>
        )}
        {(modal === 'settings' || modal === 'providers') && (
          <SettingsDialog
            workspace={w}
            close={() => setModal('')}
            initial={modal === 'providers' ? 'providers' : 'appearance'}
          />
        )}
        {modal === 'plugins' && <PluginManager workspace={w} close={() => setModal('')} />}
      </div>
    </>
  )
}

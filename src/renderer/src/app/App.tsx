import { I18nProvider, useI18n } from '../i18n'
import { PrimeLogo } from '../components/PrimeLogo'
import { useEffect, useRef, useState } from 'react'
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
import { ModelDialog } from '../features/models/ModelDialog'
import { SettingsDialog } from '../features/settings/SettingsDialog'
import { PluginManager } from '../features/plugins/PluginManager'
import { CommandPalette } from '../features/chat/CommandPalette'

export function App() {
  const w = useWorkspace()
  return (
    <I18nProvider language={w.preferences.language}>
      <WorkspaceView workspace={w} />
    </I18nProvider>
  )
}

function WorkspaceView({ workspace: w }: { workspace: Workspace }) {
  const { t, locale } = useI18n()
  const [modal, setModal] = useState('')
  const [sidebar, setSidebar] = useState(true)
  const [inspector, setInspector] = useState(true)
  const [widths, setWidths] = useState({ sidebar: 256, inspector: 334 })
  const [zoom, setZoom] = useState(1)
  const shell = useRef<HTMLDivElement>(null)
  useEffect(() => {
    setWidths({ sidebar: w.preferences.sidebarWidth, inspector: w.preferences.inspectorWidth })
  }, [w.preferences.sidebarWidth, w.preferences.inspectorWidth])
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if (event.ctrlKey && event.code === 'KeyP') {
        event.preventDefault()
        setModal('palette')
      }
      if (event.ctrlKey && event.code === 'KeyL') {
        event.preventDefault()
        if (w.project) setModal('models')
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
  const projectName = w.project?.path.split(/[\\/]/).pop()
  const isEmpty = !w.messages.length && !w.busy
  return (
    <div
      ref={shell}
      className={`app-shell ${sidebar ? '' : 'hide-sidebar'} ${inspector ? '' : 'hide-inspector'}`}
      style={
        {
          '--sidebar-width': widths.sidebar + 'px',
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
            onPalette={() => setModal('palette')}
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
            <button
              className="icon"
              aria-label={t('Поиск и команды')}
              title={t('Поиск и команды · Ctrl P')}
              onClick={() => setModal('palette')}
            >
              <Search size={17} />
            </button>
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
        {w.loading && (
          <div className="loading-banner">
            <LoaderCircle size={15} className="spin" /> {t('Открываем проект и подключаем Prime…')}
          </div>
        )}
        {isEmpty ? (
          <section className="welcome">
            <div className="welcome-wordmark">
              <PrimeLogo />
            </div>
            <div className="eyebrow">{t('ВАШ ПРОЕКТ. ВАШИ ИНСТРУМЕНТЫ.')}</div>
            <h1>{w.project ? t('Что сделаем в проекте?') : t('От идеи к работающему коду.')}</h1>
            <p>
              {w.project ? (
                <>
                  {t('Prime работает в')} <strong>{projectName}</strong>.<br />
                  {t('Опишите задачу — контекст и инструменты уже рядом.')}
                </>
              ) : (
                <>
                  {t('Привычный Prime в отдельном рабочем пространстве.')}
                  <br />
                  {t('Откройте проект, чтобы начать.')}
                </>
              )}
            </p>
            {!w.project && (
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
              {[
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
              ].map((item) => (
                <button
                  key={item.title}
                  disabled={!w.project}
                  onClick={() => void w.send(item.prompt)}
                >
                  <item.icon size={19} />
                  <strong>{item.title}</strong>
                  <span>{item.description}</span>
                  <ArrowUpRight className="starter-arrow" size={13} />
                </button>
              ))}
            </div>
            {!w.project && w.info.runtime === 'starting' && (
              <div className="startup-status">
                <LoaderCircle size={13} className="spin" /> {t('Подготавливаем встроенный Prime')}
              </div>
            )}
          </section>
        ) : (
          <MessageList messages={w.messages} busy={w.busy} />
        )}
        <Approvals workspace={w} />
        <Composer workspace={w} onModels={() => setModal('models')} />
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
          <Inspector workspace={w} close={() => setInspector(false)} />
        </>
      )}
      {modal === 'models' && (
        <ModelDialog workspace={w} close={() => setModal('')} setup={() => setModal('providers')} />
      )}
      {(modal === 'settings' || modal === 'providers') && (
        <SettingsDialog
          workspace={w}
          close={() => setModal('')}
          initial={modal === 'providers' ? 'providers' : 'appearance'}
        />
      )}
      {modal === 'plugins' && <PluginManager workspace={w} close={() => setModal('')} />}
      {modal === 'palette' && (
        <CommandPalette
          workspace={w}
          close={() => setModal('')}
          settings={() => setModal('settings')}
          models={() => setModal('models')}
          plugins={() => setModal('plugins')}
          inspector={() => setInspector((value) => !value)}
        />
      )}
    </div>
  )
}

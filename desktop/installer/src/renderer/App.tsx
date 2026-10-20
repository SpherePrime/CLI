import { useEffect, useState } from 'react'
import {
  ArrowRight,
  Check,
  FolderOpen,
  LoaderCircle,
  MessageSquare,
  Code2,
  Blocks,
  X,
  AppWindow,
  RefreshCw,
  CheckCheck
} from 'lucide-react'
import type { InstallStatus, SetupInfo } from '../shared/contracts'
import { PrimeLogo } from '../../../src/renderer/src/components/PrimeLogo'

export function SetupApp() {
  const [info, setInfo] = useState<SetupInfo>({
    version: '',
    directory: '',
    installedSize: 0,
    method: { label: '', directoryLocked: false }
  })
  const locked = info.method.directoryLocked
  const [directory, setDirectory] = useState('')
  const [status, setStatus] = useState<InstallStatus>({ phase: 'ready' })
  const [error, setError] = useState('')
  const active = ['preparing', 'installing', 'verifying', 'voice'].includes(status.phase)
  const complete = status.phase === 'complete'
  useEffect(() => {
    void window.setup
      .info()
      .then((value) => {
        setInfo(value)
        setDirectory(value.directory)
      })
      .catch((error) => setError(error.message))
    return window.setup.onProgress((value) => {
      setStatus(value)
      setError(value.error || '')
    })
  }, [])
  async function install() {
    setError('')
    try {
      setStatus(await window.setup.install(directory))
    } catch (error: any) {
      setError(error.message)
    }
  }
  const steps = [
    { phase: 'preparing', label: 'Проверка файлов' },
    { phase: 'installing', label: 'Установка приложения' },
    { phase: 'verifying', label: 'Подготовка к запуску' },
    {
      phase: 'voice',
      label:
        status.phase === 'voice' && status.percent !== undefined
          ? `Голосовая модель · ${Math.floor(status.percent)}%`
          : 'Голосовая модель'
    }
  ]
  const current = steps.findIndex((step) => step.phase === status.phase)
  return (
    <div className="setup-shell">
      <header className="setup-titlebar">
        <div className="setup-wordmark">
          <PrimeLogo />
          <span>
            Prime<span className="setup-wordmark-separator">/</span>
            <span className="setup-wordmark-detail">Desktop</span>
          </span>
        </div>
        <div className="setup-window-actions">
          <span>{info.version && `v${info.version}`}</span>
          <button
            aria-label="Закрыть установщик"
            disabled={active}
            onClick={() => void window.setup.close()}
          >
            <X size={17} />
          </button>
        </div>
      </header>
      <main className="setup-layout">
        <aside className="setup-story">
          <span className="setup-kicker">ВАШИ ИДЕИ. ВАШ PRIME.</span>
          <h1>
            От идеи
            <br />к <span>результату.</span>
          </h1>
          <p>
            Одно пространство.
            <br />
            Для всего, что вы создаёте.
          </p>
          <div className="setup-constellation" aria-hidden="true">
            <div className="setup-orbit" />
            <div className="setup-orbit inner" />
            <span className="setup-node node-chat">
              <MessageSquare size={21} />
            </span>
            <span className="setup-node node-code">
              <Code2 size={21} />
            </span>
            <span className="setup-node node-tools">
              <Blocks size={21} />
            </span>
            <div className="setup-core">
              <PrimeLogo />
            </div>
          </div>
          <div className="setup-signature">
            <span className="setup-signature-mark" />
            SPHEREPRIME
          </div>
        </aside>
        <section className="setup-content">
          <div className="setup-stage-label">
            <span />
            {complete
              ? 'ГОТОВО К РАБОТЕ'
              : active
                ? 'СОЗДАЁМ ВАШЕ ПРОСТРАНСТВО'
                : 'ПРИВЕТ, НОВЫЕ ИДЕИ'}
          </div>
          <h2>
            {complete
              ? 'Теперь всё ваше.'
              : active
                ? 'Ещё немного —\nи можно создавать.'
                : 'Добро пожаловать\nв Prime Desktop.'}
          </h2>
          <p className="setup-description">
            {complete
              ? 'Prime Desktop установлен и готов к работе. Можно начинать.'
              : active
                ? 'Устанавливаем Prime Desktop. Всё происходит автоматически.'
                : 'Ваши чаты, проекты и инструменты. Вместе, в одном приложении.'}
          </p>
          {!active && !complete && (
            <>
              <div className="setup-bundle">
                <div>
                  <span className="setup-bundle-icon">
                    <AppWindow size={19} />
                  </span>
                  <span>
                    <strong>Prime Desktop</strong>
                    <small>{info.method.label || 'Ваше рабочее пространство'}</small>
                  </span>
                  <Check size={16} />
                </div>
              </div>
              <label className="setup-folder-label" htmlFor="setup-directory">
                Папка установки
                <span>
                  {info.installedSize > 0 &&
                    `${Math.ceil(info.installedSize / 1024 / 1024)} МБ на диске`}
                </span>
              </label>
              <div className="setup-folder">
                <FolderOpen size={17} />
                <input
                  id="setup-directory"
                  aria-label="Папка установки"
                  value={directory}
                  readOnly={locked}
                  onChange={(event) => setDirectory(event.target.value)}
                  spellCheck={false}
                />
                {!locked && (
                  <button
                    aria-label="Выбрать папку"
                    onClick={() =>
                      void window.setup
                        .chooseDirectory(directory)
                        .then((value) => {
                          if (value) setDirectory(value)
                        })
                        .catch((error) => setError(error.message))
                    }
                  >
                    Изменить
                  </button>
                )}
              </div>
            </>
          )}
          {active && (
            <div className="setup-installing" role="status" aria-live="polite">
              <div className="setup-running-line">
                <span />
              </div>
              <div className="setup-steps">
                {steps.map((step, index) => (
                  <div
                    className={index === current ? 'current' : index < current ? 'done' : ''}
                    key={step.phase}
                  >
                    <span>
                      {index < current ? (
                        <Check size={14} />
                      ) : index === current ? (
                        <LoaderCircle size={14} className="setup-spin" />
                      ) : (
                        <i />
                      )}
                    </span>
                    <strong>{step.label}</strong>
                  </div>
                ))}
              </div>
              <p>Окно можно оставить открытым — мы всё подготовим.</p>
            </div>
          )}
          {complete && (
            <div className="setup-complete">
              <span>
                <CheckCheck size={30} />
              </span>
              <div>
                <strong>Всё на месте</strong>
                <p>{status.warning || 'Приложение, голосовая модель и ярлыки готовы.'}</p>
              </div>
            </div>
          )}
          {error && (
            <p className="setup-error" role="alert">
              {error}
            </p>
          )}
          <footer className="setup-content-footer">
            {complete ? (
              <button
                className="setup-primary"
                onClick={() => void window.setup.open().catch((error) => setError(error.message))}
              >
                Открыть Prime
                <ArrowRight size={19} />
              </button>
            ) : !active ? (
              <button
                className="setup-primary"
                disabled={!directory.trim()}
                onClick={() => void install()}
              >
                {status.phase === 'error' ? 'Попробовать ещё раз' : 'Установить Prime'}
                {status.phase === 'error' ? <RefreshCw size={18} /> : <ArrowRight size={19} />}
              </button>
            ) : (
              <div className="setup-working">
                <span className="setup-working-dot" />
                Prime скоро будет готов
              </div>
            )}
          </footer>
        </section>
      </main>
    </div>
  )
}

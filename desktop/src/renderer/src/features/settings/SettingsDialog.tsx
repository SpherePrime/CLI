import { PrimeSettings } from './PrimeSettings'
import { IntegrationSettings } from './IntegrationSettings'
import { ProviderSettings } from './ProviderSettings'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import { PrimeLogo } from '../../components/PrimeLogo'
import { useState } from 'react'
import {
  Check,
  Boxes,
  Code2,
  BookOpen,
  Monitor,
  Server,
  SlidersHorizontal,
  Info,
  Sun,
  Moon,
  Download
} from 'lucide-react'
import { Modal } from '../../components/Modal'
import type { Workspace } from '../../app/useWorkspace'
export function SettingsDialog({
  workspace: w,
  close,
  initial = 'appearance'
}: {
  workspace: Workspace
  close: () => void
  initial?: string
}) {
  const { t, language } = useI18n()
  const [tab, setTab] = useState(initial)
  const [status, setStatus] = useState('')
  const [scope] = useState(0)
  const tabs = [
    { id: 'appearance', label: t('Интерфейс'), icon: Monitor },
    { id: 'providers', label: t('Провайдеры'), icon: Server },
    { id: 'mcp', label: 'MCP', icon: Boxes },
    { id: 'lsp', label: 'LSP', icon: Code2 },
    { id: 'skills', label: 'Skills', icon: BookOpen },
    { id: 'advanced', label: t('Настройки Prime'), icon: SlidersHorizontal },
    { id: 'about', label: t('О приложении'), icon: Info }
  ]
  return (
    <Modal title={t('Настройки')} close={close} wide className="settings-modal">
      <div className="settings-layout">
        <nav>
          {tabs.map((item) => (
            <button
              key={item.id}
              className={tab === item.id ? 'active' : ''}
              onClick={() => {
                setTab(item.id)
                setStatus('')
              }}
            >
              <item.icon size={16} />
              {item.label}
            </button>
          ))}
        </nav>
        <div className="settings-body">
          {tab === 'appearance' && (
            <>
              <h3>{t('Ваше рабочее пространство')}</h3>
              <p className="muted">{t('Оформление приложения сохраняется на этом устройстве.')}</p>
              <label className="form-label">
                {t('Язык интерфейса')}
                <Select
                  label={t('Язык интерфейса')}
                  value={language}
                  onChange={(value) => {
                    void w.savePreferences({ language: value as 'ru' | 'en' })
                  }}
                  options={[
                    { value: 'ru', label: t('Русский') },
                    { value: 'en', label: t('Английский') }
                  ]}
                />
              </label>
              <label className="form-label">{t('Цветовая тема')}</label>
              <div className="theme-options">
                {(
                  [
                    { id: 'dark', label: t('Тёмная'), icon: Moon },
                    { id: 'light', label: t('Светлая'), icon: Sun },
                    { id: 'system', label: t('Системная'), icon: Monitor }
                  ] as const
                ).map((theme) => (
                  <button
                    key={theme.id}
                    className={w.preferences.theme === theme.id ? 'selected' : ''}
                    onClick={() => void w.savePreferences({ theme: theme.id })}
                  >
                    <theme.icon size={22} />
                    {theme.label}
                    {w.preferences.theme === theme.id && <Check size={13} />}
                  </button>
                ))}
              </div>
              <div className="settings-note">
                <strong>{t('Клавиатура')}</strong>
                <p>
                  {t('Ctrl+N — новый чат · Ctrl+L — модели')}
                  <br />
                  {t('Shift+Enter — новая строка · двойной Esc — остановить задачу')}
                  <br />
                  {t('Alt+V — голосовой ввод')}
                </p>
              </div>
            </>
          )}
          {tab === 'providers' && <ProviderSettings workspace={w} scope={scope} />}
          {tab === 'advanced' && <PrimeSettings workspace={w} />}
          {(['mcp', 'lsp', 'skills'] as const).map(
            (kind) => tab === kind && <IntegrationSettings key={kind} kind={kind} workspace={w} />
          )}
          {tab === 'about' && (
            <>
              <div className="about-symbol">
                <PrimeLogo />
              </div>
              <h3>Prime Desktop</h3>
              <p className="muted">{t('Локальное рабочее пространство для Prime.')}</p>
              <dl className="info-list">
                <dt>{t('Приложение')}</dt>
                <dd>{w.info.version || '0.1.0'}</dd>
                <dt>{t('Компоненты приложения')}</dt>
                <dd>{w.info.runtimeVersion?.version || t('Запуск…')}</dd>
                <dt>{t('Среда выполнения')}</dt>
                <dd>
                  {(
                    {
                      ready: t('Готов'),
                      starting: t('Запуск…'),
                      error: t('Ошибка'),
                      stopped: t('Остановлен')
                    } as Record<string, string>
                  )[w.info.runtime] || w.info.runtime}
                </dd>
                <dt>{t('Соединение')}</dt>
                <dd>Windows named pipe</dd>
              </dl>
              <button
                onClick={async () => {
                  const result = await window.prime.update('check')
                  setStatus(
                    result.message ||
                      (result.state === 'current'
                        ? t('Установлена актуальная версия')
                        : t('Обновление: {value0}', { value0: result.version || result.state }))
                  )
                }}
              >
                <Download size={15} /> {t('Проверить обновления')}
              </button>
              <p className="muted small-text">
                {t('Обновления Prime Desktop загружаются из GitHub Releases.')}
              </p>
              {w.info.error && <pre>{w.info.error}</pre>}
            </>
          )}
          {status && (
            <p className="settings-status" role="status">
              {status}
            </p>
          )}
        </div>
      </div>
    </Modal>
  )
}

import { ShortcutSettings } from '../shortcuts/ShortcutSettings'
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
  Download,
  AudioLines,
  Keyboard,
  Image,
  KeyRound
} from 'lucide-react'
import { VoiceSettings } from './VoiceSettings'
import { ImageGenSettings } from './ImageGenSettings'
import { SudoSettings } from './SudoSettings'
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
    { id: 'shortcuts', label: t('Горячие клавиши'), icon: Keyboard },
    { id: 'providers', label: t('Провайдеры'), icon: Server },
    { id: 'voice', label: t('Голос и речь'), icon: AudioLines },
    { id: 'images', label: t('Изображения'), icon: Image },
    { id: 'sudo', label: t('Sudo'), icon: KeyRound },
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
              aria-current={tab === item.id ? 'page' : undefined}
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
        <div className="settings-body" key={tab}>
          {tab === 'voice' && <VoiceSettings workspace={w} />}
          {tab === 'images' && <ImageGenSettings workspace={w} />}
          {tab === 'sudo' && <SudoSettings />}
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
                <strong>{t('Плотность интерфейса')}</strong>
                <div
                  className="density-options"
                  role="group"
                  aria-label={t('Плотность интерфейса')}
                >
                  {(
                    [
                      { id: 'comfortable', label: t('Просторная') },
                      { id: 'compact', label: t('Компактная') }
                    ] as const
                  ).map((option) => (
                    <button
                      key={option.id}
                      aria-pressed={(w.preferences.density || 'comfortable') === option.id}
                      onClick={() =>
                        void w.savePreferences({ density: option.id }).catch(w.errorHandler)
                      }
                    >
                      <span className={`density-preview ${option.id}`} aria-hidden="true">
                        <i />
                        <i />
                        <i />
                      </span>
                      {option.label}
                      {(w.preferences.density || 'comfortable') === option.id && (
                        <Check size={14} />
                      )}
                    </button>
                  ))}
                </div>
                <label className="setting-toggle motion-setting">
                  <span>
                    <strong>{t('Меньше анимаций')}</strong>
                    <small>{t('Спокойные переходы без движения и пульсации')}</small>
                  </span>
                  <input
                    type="checkbox"
                    checked={w.preferences.reduceMotion || false}
                    onChange={(event) =>
                      void w
                        .savePreferences({ reduceMotion: event.target.checked })
                        .catch(w.errorHandler)
                    }
                  />
                </label>
              </div>
            </>
          )}
          {tab === 'shortcuts' && <ShortcutSettings workspace={w} />}
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
                <dd>{w.info.host === 'tauri' ? 'Rust / Go · in-process' : 'Windows named pipe'}</dd>
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
            <p
              className={`settings-status ${status === t('Сохранено') ? 'success' : ''}`}
              role="status"
            >
              {status}
            </p>
          )}
        </div>
      </div>
    </Modal>
  )
}

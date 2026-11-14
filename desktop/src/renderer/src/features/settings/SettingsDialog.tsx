import { ShortcutSettings } from '../shortcuts/ShortcutSettings'
import { PrimeSettings } from './PrimeSettings'
import { IntegrationSettings } from './IntegrationSettings'
import { ProviderSettings } from './ProviderSettings'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import { PrimeLogo } from '../../components/PrimeLogo'
import { useId, useState } from 'react'
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
  KeyRound,
  UserRound
} from 'lucide-react'
import { VoiceSettings } from './VoiceSettings'
import { SudoSettings } from './SudoSettings'
import { AccountCard } from '../auth/AccountCard'
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
  const [updateStatus, setUpdateStatus] = useState('')
  const languageLabelId = useId()
  const themeLabelId = useId()
  const tabs = [
    { id: 'account', label: t('Аккаунт'), icon: UserRound },
    { id: 'appearance', label: t('Интерфейс'), icon: Monitor },
    { id: 'shortcuts', label: t('Горячие клавиши'), icon: Keyboard },
    { id: 'providers', label: t('Провайдеры'), icon: Server },
    { id: 'voice', label: t('Голос и речь'), icon: AudioLines },
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
                setUpdateStatus('')
              }}
            >
              <span className="settings-nav-icon" aria-hidden="true">
                <item.icon size={15} />
              </span>
              <span className="settings-nav-label">{item.label}</span>
            </button>
          ))}
        </nav>
        <div className="settings-body" key={tab}>
          {tab === 'account' && (
            <AccountCard
              account={w.account}
              limits={w.primeLimits}
              onRefreshLimits={() => w.refreshPrimeLimits(true)}
              onLogout={w.logoutAccount}
              onAfterLogout={close}
            />
          )}
          {tab === 'voice' && <VoiceSettings workspace={w} />}
          {tab === 'sudo' && <SudoSettings />}
          {tab === 'appearance' && (
            <>
              <h3>{t('Ваше рабочее пространство')}</h3>
              <p className="muted">{t('Оформление приложения сохраняется на этом устройстве.')}</p>
              <section className="settings-card">
                <div className="form-label">
                  <span id={languageLabelId}>{t('Язык интерфейса')}</span>
                  <Select
                    label={t('Язык интерфейса')}
                    labelledBy={languageLabelId}
                    value={language}
                    onChange={(value) => {
                      void w
                        .savePreferences({ language: value as 'ru' | 'en' })
                        .catch(w.errorHandler)
                    }}
                    options={[
                      { value: 'ru', label: t('Русский') },
                      { value: 'en', label: t('Английский') }
                    ]}
                  />
                </div>
              </section>
              <section className="settings-card">
                <div className="form-label" id={themeLabelId}>
                  {t('Цветовая тема')}
                </div>
                <div className="theme-options" role="group" aria-labelledby={themeLabelId}>
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
                      aria-pressed={w.preferences.theme === theme.id}
                      onClick={() =>
                        void w.savePreferences({ theme: theme.id }).catch(w.errorHandler)
                      }
                    >
                      <span className={`theme-preview ${theme.id}`} aria-hidden="true">
                        <i />
                        <i />
                        <i />
                      </span>
                      <span className="theme-option-label">
                        <theme.icon size={15} />
                        {theme.label}
                      </span>
                      {w.preferences.theme === theme.id && <Check size={13} />}
                    </button>
                  ))}
                </div>
              </section>
              <section className="settings-card settings-note">
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
              </section>
            </>
          )}
          {tab === 'shortcuts' && <ShortcutSettings workspace={w} />}
          {tab === 'providers' && <ProviderSettings workspace={w} scope={0} />}
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
                  try {
                    const result = await window.prime.update('check')
                    setUpdateStatus(
                      result.message ||
                        (result.state === 'current'
                          ? t('Установлена актуальная версия')
                          : t('Обновление: {value0}', { value0: result.version || result.state }))
                    )
                  } catch (error) {
                    w.errorHandler(error)
                  }
                }}
              >
                <Download size={15} /> {t('Проверить обновления')}
              </button>
              <p className="muted small-text">
                {t('Обновления Prime Desktop загружаются из GitHub Releases.')}
              </p>
              {w.info.error && <pre>{w.info.error}</pre>}
              {updateStatus && (
                <p className="settings-status" role="status">
                  {updateStatus}
                </p>
              )}
            </>
          )}
        </div>
      </div>
    </Modal>
  )
}

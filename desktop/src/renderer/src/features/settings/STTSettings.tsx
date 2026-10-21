import { useCallback, useEffect, useState } from 'react'
import { Check, Download, Trash2 } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import type { SttModel, SttProvider, SttStatus, SttTask } from '../../../../shared/stt'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'

const PROVIDER_LABELS: Record<string, string> = {
  google: 'Google STT',
  whisper: 'Whisper STT',
  vosk: 'Vosk STT'
}

const LANGUAGE_LABELS: Record<string, string> = { ru: 'Русский', en: 'Английский' }

export function STTSettings({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [status, setStatus] = useState<SttStatus>()
  const [tasks, setTasks] = useState<SttTask[]>([])
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const report = (failure: unknown) =>
    setError(String((failure as Error)?.message || failure).replace(/^.*Error: /, ''))
  const taskName = (provider: string, model: string) => `stt:${provider}:${model}`
  const refresh = useCallback(async () => {
    try {
      const [next, list] = await Promise.all([w.call('stt'), w.call('sttTasks')])
      setStatus(next as SttStatus)
      setTasks(Array.isArray(list) ? (list as SttTask[]) : [])
    } catch (failure) {
      report(failure)
    }
  }, [w])
  useEffect(() => {
    void refresh()
  }, [refresh, w.project?.id])
  const running = tasks.some((task) => task.state === 'running')
  useEffect(() => {
    if (!running) return
    const timer = setInterval(() => void refresh(), 1500)
    return () => clearInterval(timer)
  }, [running, refresh])
  const run = async (key: string, action: () => Promise<unknown>) => {
    setError('')
    setBusy(key)
    try {
      await action()
      await refresh()
    } catch (failure) {
      report(failure)
    } finally {
      setBusy('')
    }
  }
  const pickProvider = (provider: SttProvider) =>
    run(provider.id, async () => {
      const model =
        provider.id === 'google'
          ? ''
          : provider.models.find((item) => item.active)?.id ||
            provider.models.find((item) => item.installed)?.id ||
            provider.models.find((item) => item.recommended)?.id ||
            provider.models[0]?.id ||
            ''
      await w.call('sttSelect', { provider: provider.id, model })
    })
  const install = (provider: SttProvider, model: SttModel) =>
    run(taskName(provider.id, model.id), () =>
      w.call('sttInstall', { provider: provider.id, model: model.id })
    )
  const remove = (provider: SttProvider, model: SttModel) =>
    run(taskName(provider.id, model.id), () =>
      w.call('sttRemove', { provider: provider.id, model: model.id })
    )
  const choose = (provider: SttProvider, model: SttModel) =>
    run(taskName(provider.id, model.id), () =>
      w.call('sttSelect', { provider: provider.id, model: model.id })
    )
  const formatSize = (bytes: number) =>
    bytes >= 1024 ** 3
      ? t('{value0} ГБ', { value0: (bytes / 1024 ** 3).toFixed(2) })
      : t('{value0} МБ', { value0: Math.round(bytes / 1024 ** 2) })
  const provider = status?.providers.find((item) => item.id === status.engine)
  const engineLabel = provider?.title || status?.engine || ''
  const local = provider && provider.id !== 'google' ? provider : undefined
  const sectionTitle =
    local?.id === 'whisper' ? t('Модели Whisper') : local?.id === 'vosk' ? t('Модели Vosk') : ''
  const options = (status?.providers || []).map((item) => ({
    value: item.id,
    label: t(PROVIDER_LABELS[item.id] || item.title)
  }))
  const value = options.some((item) => item.value === status?.engine)
    ? (status?.engine as string)
    : options[0]?.value || ''
  return (
    <>
      <h3>{t('Распознавание речи')}</h3>
      <p className="muted">
        {t('Распознавание речи для диктовки: облачный Google или локальные модели Whisper и Vosk.')}
      </p>
      <p className="muted" role="status">
        {t('Провайдер: {value0}', { value0: engineLabel })}
      </p>
      {status && (
        <label className="form-label">
          {t('Провайдер распознавания речи')}
          <Select
            label={t('Провайдер распознавания речи')}
            value={value}
            disabled={busy !== '' || running}
            onChange={(next) => {
              const nextProvider = status.providers.find((item) => item.id === next)
              if (nextProvider) void pickProvider(nextProvider)
            }}
            options={options}
          />
        </label>
      )}
      {provider && <p className="muted">{provider.description}</p>}
      {local && local.models.every((item) => !item.installed) && (
        <p className="muted">{t('Скачайте модель и выберите её')}</p>
      )}
      {provider?.id === 'google' && (
        <p className="muted">{t('Работает онлайн без установки моделей.')}</p>
      )}
      {sectionTitle && <h4>{sectionTitle}</h4>}
      {local &&
        local.models.map((model) => {
          const name = taskName(local.id, model.id)
          const task = tasks.find((item) => item.name === name && item.state === 'running')
          const failed = tasks.find((item) => item.name === name && item.state === 'failed')
          const percent = task && task.total > 0 ? Math.floor((task.done / task.total) * 100) : 0
          return (
            <div className="stt-model-row" key={model.id}>
              <div>
                <h3>
                  {model.label}
                  {model.recommended && <span className="stt-badge">{t('Рекомендуется')}</span>}
                </h3>
                <p>
                  {formatSize(model.size_bytes)}
                  {local.id === 'vosk' &&
                    model.language &&
                    ` · ${t(LANGUAGE_LABELS[model.language] || model.language)}`}
                </p>
                {task && (
                  <div className="stt-progress">
                    <progress value={task.done} max={task.total || 1} />
                    <small>{t('{value0}%', { value0: percent })}</small>
                  </div>
                )}
                {failed?.error && <p className="error-text">{failed.error}</p>}
              </div>
              <div className="stt-model-actions">
                {task ? null : !model.installed ? (
                  <button
                    disabled={busy === name || running}
                    onClick={() => void install(local, model)}
                  >
                    <Download size={14} />
                    {t('Скачать')}
                  </button>
                ) : model.active ? (
                  <>
                    <small>{t('Активна')}</small>
                    <button
                      className="quiet"
                      disabled={busy === name || running}
                      onClick={() => void remove(local, model)}
                    >
                      <Trash2 size={14} />
                      {t('Удалить')}
                    </button>
                  </>
                ) : (
                  <>
                    <button
                      className="primary"
                      disabled={busy === name || running}
                      onClick={() => void choose(local, model)}
                    >
                      <Check size={14} />
                      {t('Выбрать')}
                    </button>
                    <button
                      className="quiet"
                      disabled={busy === name || running}
                      onClick={() => void remove(local, model)}
                    >
                      <Trash2 size={14} />
                      {t('Удалить')}
                    </button>
                  </>
                )}
              </div>
            </div>
          )
        })}
      {error && <p className="error-text">{error}</p>}
    </>
  )
}

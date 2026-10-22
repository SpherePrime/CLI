import { useCallback, useEffect, useRef, useState } from 'react'
import { Check, Download, Play, Trash2, Upload } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import type { SpeechStatus, SpeechVoice } from '../../../../shared/speech'
import type { SttModel, SttProvider, SttStatus, SttTask } from '../../../../shared/stt'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import { Speaker } from '../voice/speaker'
import { useAudioDevices } from './useAudioDevices'
import '../../styles/conversation.css'

const PROVIDER_LABELS: Record<string, string> = {
  google: 'Google STT',
  whisper: 'Whisper STT',
  vosk: 'Vosk STT'
}

const LANGUAGES: Record<string, string> = {
  ru: 'Русский',
  en: 'Английский',
  de: 'Немецкий',
  es: 'Испанский',
  fr: 'Французский',
  it: 'Итальянский',
  pt: 'Португальский',
  zh: 'Китайский',
  ja: 'Японский',
  uk: 'Украинский'
}

export function VoiceSettings({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [voices, setVoices] = useState<SpeechVoice[]>([])
  const [status, setStatus] = useState<SpeechStatus>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const speaker = useRef<Speaker>(undefined)
  const voice = w.preferences.speechVoice ?? 'F1'
  const speed = w.preferences.speechSpeed ?? 1.05
  const micDeviceId = w.preferences.micDeviceId ?? ''
  const speakerDeviceId = w.preferences.speakerDeviceId ?? ''
  const { inputs, outputs, defaultInput, defaultOutput } = useAudioDevices()
  const autoInput = defaultInput ? t('Авто - {value0}', { value0: defaultInput }) : t('Авто')
  const autoOutput = defaultOutput ? t('Авто - {value0}', { value0: defaultOutput }) : t('Авто')
  const report = (failure: unknown) =>
    setError(String((failure as Error)?.message || failure).replace(/^.*Error: /, ''))
  useEffect(() => {
    void window.prime.speech({ action: 'voices' }).then(setVoices).catch(report)
    void window.prime.speech({ action: 'status' }).then(setStatus).catch(report)
    const unsubscribe = window.prime.onEvent((event) => {
      if (event.type === 'speech') setStatus(event.payload)
    })
    return () => {
      unsubscribe()
      void speaker.current?.dispose()
    }
  }, [])
  const preview = async () => {
    setError('')
    setBusy(true)
    try {
      speaker.current?.stop()
      // A device change needs a fresh AudioContext, so the speaker is rebuilt
      // when the output selection differs from the one it was built with.
      if (speaker.current && speaker.current.deviceId !== speakerDeviceId) {
        await speaker.current.dispose()
        speaker.current = undefined
      }
      speaker.current ??= new Speaker({
        voice,
        speed,
        deviceId: speakerDeviceId || undefined,
        onState: () => {},
        onError: report
      })
      speaker.current.update({ voice, speed, deviceId: speakerDeviceId || undefined })
      await speaker.current.speak([t('Привет! Так я буду отвечать в голосовом разговоре.')])
    } finally {
      setBusy(false)
    }
  }
  const current = voices.find((item) => item.id === voice)
  return (
    <>
      <p className="muted">
        {t('Голосовой ввод и озвучка ответов: микрофон, распознавание речи и локальный синтез.')}
      </p>
      <h3>{t('Устройства')}</h3>
      <label className="form-label">
        {t('Микрофон')}
        <Select
          label={t('Микрофон')}
          value={micDeviceId}
          onChange={(value) => void w.savePreferences({ micDeviceId: value }).catch(report)}
          options={[
            { value: '', label: autoInput },
            ...inputs.map((device) => ({ value: device.id, label: device.label }))
          ]}
        />
      </label>
      <label className="form-label">
        {t('Вывод звука')}
        <Select
          label={t('Вывод звука')}
          value={speakerDeviceId}
          onChange={(value) => void w.savePreferences({ speakerDeviceId: value }).catch(report)}
          options={[
            { value: '', label: autoOutput },
            ...outputs.map((device) => ({ value: device.id, label: device.label }))
          ]}
        />
      </label>
      <h3>{t('Распознавание речи')}</h3>
      <SttSection workspace={w} />
      <h3>{t('Озвучка ответов')}</h3>
      <p className="muted">
        {t('Локальная модель. Речь синтезируется на этом компьютере без отправки в интернет.')}
      </p>
      <p className="muted" role="status">
        {status?.state === 'downloading'
          ? t('Загрузка голосовой модели · {value0}%', { value0: Math.floor(status.percent ?? 0) })
          : status?.state === 'missing'
            ? t('Модель ещё не скачана. Она загрузится при первом разговоре.')
            : status?.state === 'error'
              ? status.error
              : status?.state === 'loading'
                ? t('Загрузка модели…')
                : t('Модель готова')}
      </p>
      <label className="form-label">
        {t('Голос ответа')}
        <Select
          label={t('Голос ответа')}
          value={voice}
          onChange={(value) => void w.savePreferences({ speechVoice: value }).catch(report)}
          options={voices.map((item) => ({
            value: item.id,
            label: item.custom ? item.name : t(item.name)
          }))}
        />
      </label>
      <label className="form-label">
        {t('Скорость речи · {value0}×', { value0: speed.toFixed(2) })}
        <input
          type="range"
          min={0.8}
          max={1.4}
          step={0.05}
          value={speed}
          onChange={(event) =>
            void w.savePreferences({ speechSpeed: Number(event.target.value) }).catch(report)
          }
        />
      </label>
      <div className="settings-actions">
        <button disabled={busy} onClick={() => void preview()}>
          <Play size={14} />
          {t('Прослушать')}
        </button>
        <button
          onClick={() =>
            void window.prime
              .speech({ action: 'import' })
              .then(async (added: SpeechVoice | null) => {
                setVoices(await window.prime.speech({ action: 'voices' }))
                if (added) await w.savePreferences({ speechVoice: added.id })
              })
              .catch(report)
          }
        >
          <Upload size={14} />
          {t('Импортировать голос')}
        </button>
        {current?.custom && (
          <button
            onClick={() =>
              void window.prime
                .speech({ action: 'remove', voice: current.id })
                .then(async (list: SpeechVoice[]) => {
                  setVoices(list)
                  await w.savePreferences({ speechVoice: 'F1' })
                })
                .catch(report)
            }
          >
            <Trash2 size={14} />
            {t('Удалить голос')}
          </button>
        )}
      </div>
      <p className="muted">
        {t('Импорт принимает JSON-стиль голоса (например, из Voice Builder).')}
      </p>
      {error && <p className="error-text">{error}</p>}
    </>
  )
}

function SttSection({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [status, setStatus] = useState<SttStatus>()
  const [tasks, setTasks] = useState<SttTask[]>([])
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  // The catalog does not echo the exact configured GPU index, so the last user
  // choice is remembered locally (scoped to the provider) for display only.
  const [picked, setPicked] = useState<{ for: string; id: string } | null>(null)
  const [accelChoice, setAccelChoice] = useState<{ for: string; value: string } | null>(null)
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
  const running = tasks.some((item) => item.state === 'running')
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
  const engine = status?.engine || ''
  const language = status?.language ?? ''
  const provider = status?.providers.find((item) => item.id === engine)
  const local = provider && provider.id !== 'google' ? provider : undefined
  const models = local?.models ?? []
  // Vosk ships one model per language, so only models of the chosen language
  // make sense once it is fixed; the full list stays visible in auto mode.
  const pool =
    local?.id === 'vosk' && language ? models.filter((item) => item.language === language) : models
  const options = pool.length ? pool : models
  const pickedId = picked && picked.for === engine ? picked.id : undefined
  const selected =
    (pickedId ? options.find((item) => item.id === pickedId) : undefined) ??
    options.find((item) => item.active) ??
    options.find((item) => item.recommended) ??
    options[0]
  const selectedTaskName = local && selected ? taskName(local.id, selected.id) : ''
  const task = tasks.find((item) => item.name === selectedTaskName && item.state === 'running')
  const failed = tasks.find((item) => item.name === selectedTaskName && item.state === 'failed')
  const percent = task && task.total > 0 ? Math.floor((task.done / task.total) * 100) : 0
  const background = local
    ? tasks.filter(
        (item) =>
          item.state === 'running' &&
          item.name.startsWith(`stt:${local.id}:`) &&
          item.name !== selectedTaskName
      )
    : []
  const formatSize = (bytes: number) =>
    bytes >= 1024 ** 3
      ? t('{value0} ГБ', { value0: (bytes / 1024 ** 3).toFixed(2) })
      : t('{value0} МБ', { value0: Math.round(bytes / 1024 ** 2) })
  const modelLabel = (model: SttModel) =>
    `${model.recommended ? '★ ' : ''}${model.label} · ${formatSize(model.size_bytes)}${
      model.installed ? '' : ` (${t('не скачана')})`
    }`
  const providerOptions = (status?.providers || []).map((item) => ({
    value: item.id,
    label: t(PROVIDER_LABELS[item.id] || item.title)
  }))
  const languageOptions = [
    { value: '', label: t('Авто') },
    ...Object.entries(LANGUAGES).map(([code, name]) => ({ value: code, label: t(name) }))
  ]
  const gpuOptions = (local?.gpus ?? []).map((gpu) => ({
    // The backend prefix keeps NVIDIA cards listed under both CUDA and
    // Vulkan distinguishable, since the two backends number devices
    // independently.
    value: `${gpu.kind}:${gpu.index}`,
    label: `${gpu.name} (${gpu.kind.toUpperCase()})`
  }))
  const acceleratorOptions = [
    { value: 'cpu', label: t('Процессор (CPU)') },
    ...gpuOptions.filter((gpu) => local?.accelerators?.includes(gpu.value.split(':')[0]))
  ]
  // The exact stored GPU index is not echoed back, so a GPU selection simply
  // highlights the first matching option; the picked value is sent to the
  // backend.
  const acceleratorChoice =
    accelChoice && accelChoice.for === engine ? accelChoice.value : local?.accelerator || 'cpu'
  const acceleratorValue =
    acceleratorChoice !== 'cpu' && acceleratorOptions.length
      ? acceleratorChoice.includes(':')
        ? acceleratorChoice
        : (acceleratorOptions.find((option) => option.value.startsWith(`${acceleratorChoice}:`))
            ?.value ?? 'cpu')
      : 'cpu'
  const pickProvider = (next: SttProvider) =>
    run(next.id, async () => {
      const model =
        next.id === 'google'
          ? ''
          : next.models.find((item) => item.active)?.id ||
            next.models.find((item) => item.installed)?.id ||
            next.models.find((item) => item.recommended)?.id ||
            next.models[0]?.id ||
            ''
      await w.call('sttSelect', { provider: next.id, model, language })
    })
  const changeLanguage = (value: string) =>
    void run('language', () =>
      w.call('sttSelect', { provider: engine, model: status?.model ?? '', language: value })
    )
  const changeAccelerator = (value: string) => {
    if (!local) return
    const [backend, index] = value.split(':')
    const next = backend === 'cpu' || !index ? 'cpu' : backend
    setAccelChoice({ for: engine, value: next === 'cpu' ? 'cpu' : value })
    void run('accel', () =>
      w.call('sttSelect', {
        provider: local.id,
        model: status?.model ?? '',
        language,
        accelerator: next,
        gpu_device: next === 'cpu' ? '' : index
      })
    )
  }
  const pickModel = (model: SttModel) => {
    if (!local) return
    setPicked({ for: local.id, id: model.id })
    void run(taskName(local.id, model.id), async () => {
      await w.call('sttSelect', { provider: local.id, model: model.id, language })
      // Download and use: an uninstalled model starts downloading right away.
      if (!model.installed) await w.call('sttInstall', { provider: local.id, model: model.id })
    })
  }
  const install = (model: SttModel) => {
    if (!local) return
    void run(taskName(local.id, model.id), () =>
      w.call('sttInstall', { provider: local.id, model: model.id })
    )
  }
  const choose = (model: SttModel) => {
    if (!local) return
    void run(taskName(local.id, model.id), () =>
      w.call('sttSelect', { provider: local.id, model: model.id, language })
    )
  }
  const remove = (model: SttModel) => {
    if (!local) return
    void run(taskName(local.id, model.id), () =>
      w.call('sttRemove', { provider: local.id, model: model.id })
    )
  }
  return (
    <>
      {status && (
        <>
          <label className="form-label">
            {t('Провайдер распознавания речи')}
            <Select
              label={t('Провайдер распознавания речи')}
              value={
                providerOptions.some((item) => item.value === engine)
                  ? engine
                  : providerOptions[0]?.value || ''
              }
              disabled={busy !== '' || running}
              onChange={(next) => {
                const nextProvider = status.providers.find((item) => item.id === next)
                if (nextProvider) void pickProvider(nextProvider)
              }}
              options={providerOptions}
            />
          </label>
          {provider && <p className="muted">{provider.description}</p>}
          <label className="form-label">
            {t('Язык распознавания')}
            <Select
              label={t('Язык распознавания')}
              value={language}
              disabled={busy !== '' || running}
              onChange={changeLanguage}
              options={languageOptions}
            />
          </label>
          {local?.id === 'whisper' && (
            <label className="form-label">
              {t('Ускорение')}
              <Select
                label={t('Ускорение')}
                value={acceleratorValue}
                disabled={busy !== '' || running}
                onChange={changeAccelerator}
                options={acceleratorOptions}
              />
            </label>
          )}
          {options.length > 0 && (
            <label className="form-label">
              {t('Модель')}
              <Select
                label={t('Модель')}
                value={selected?.id ?? ''}
                disabled={busy !== '' || running}
                onChange={(next) => {
                  const model = options.find((item) => item.id === next)
                  if (model) pickModel(model)
                }}
                options={options.map((item) => ({ value: item.id, label: modelLabel(item) }))}
              />
            </label>
          )}
        </>
      )}
      {local && selected && task && (
        <div className="stt-progress">
          <progress value={task.done} max={task.total || 1} />
          <small>{t('{value0}%', { value0: percent })}</small>
        </div>
      )}
      {local && selected && failed?.error && <p className="error-text">{failed.error}</p>}
      {local && selected && !task && (
        <div className="settings-actions">
          {!selected.installed ? (
            <button disabled={busy !== '' || running} onClick={() => void install(selected)}>
              <Download size={14} />
              {t('Скачать')}
            </button>
          ) : selected.active ? (
            <>
              <small>{t('Активна')}</small>
              <button
                className="quiet"
                disabled={busy !== '' || running}
                onClick={() => void remove(selected)}
              >
                <Trash2 size={14} />
                {t('Удалить')}
              </button>
            </>
          ) : (
            <>
              <button
                className="primary"
                disabled={busy !== '' || running}
                onClick={() => void choose(selected)}
              >
                <Check size={14} />
                {t('Выбрать')}
              </button>
              <button
                className="quiet"
                disabled={busy !== '' || running}
                onClick={() => void remove(selected)}
              >
                <Trash2 size={14} />
                {t('Удалить')}
              </button>
            </>
          )}
        </div>
      )}
      {background.map((item) => {
        const modelId = item.name.split(':')[2] || ''
        const model = models.find((entry) => entry.id === modelId)
        const share = item.total > 0 ? Math.floor((item.done / item.total) * 100) : 0
        return (
          <div className="stt-progress" key={item.name}>
            <small>{model?.label || item.name}</small>
            <progress value={item.done} max={item.total || 1} />
            <small>{t('{value0}%', { value0: share })}</small>
          </div>
        )
      })}
      {provider?.id === 'google' && (
        <p className="muted">{t('Работает онлайн без установки моделей.')}</p>
      )}
      {error && <p className="error-text">{error}</p>}
    </>
  )
}

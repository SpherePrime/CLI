import { useCallback, useEffect, useRef, useState } from 'react'
import { Play, RefreshCw, Trash2, Upload } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import type { SpeechStatus, SpeechVoice } from '../../../../shared/speech'
import {
  normalizeAsrLanguage,
  type AsrEvent,
  type AsrLanguage,
  type AsrStatus,
  type SttStatus
} from '../../../../shared/stt'
import { Select } from '../../components/Select'
import { RangeField } from '../../components/RangeField'
import { useI18n } from '../../i18n'
import { Speaker } from '../voice/speaker'
import { useAudioDevices } from './useAudioDevices'
import '../../styles/conversation.css'

const ASR_LANGUAGE_OPTIONS: { value: AsrLanguage; label: string }[] = [
  { value: 'auto', label: 'Авто' },
  { value: 'ru', label: 'Русский' },
  { value: 'en', label: 'Английский' },
  { value: 'uk', label: 'Украинский' }
]

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
      if ('type' in event && event.type === 'speech') setStatus(event.payload)
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
      <RangeField
        label={t('Скорость речи')}
        min={0.8}
        max={1.4}
        step={0.05}
        value={speed}
        formatValue={(value) => `${value.toFixed(2)}×`}
        onChange={(value) => void w.savePreferences({ speechSpeed: value }).catch(report)}
      />
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
  const [language, setLanguage] = useState<AsrLanguage>('auto')
  const [asrStatus, setAsrStatus] = useState<AsrStatus>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const call = w.call

  const report = (failure: unknown) =>
    setError(String((failure as Error)?.message || failure).replace(/^.*Error: /, ''))

  const refresh = useCallback(async () => {
    try {
      const [catalog, nativeStatus] = await Promise.all([
        call('stt').catch(() => null) as Promise<SttStatus | null>,
        window.prime.asr({ action: 'status' }).catch(() => null) as Promise<AsrStatus | null>
      ])
      if (catalog?.language) setLanguage(normalizeAsrLanguage(catalog.language))
      if (nativeStatus) setAsrStatus(nativeStatus)
      setError('')
    } catch (failure) {
      report(failure)
    }
  }, [call])

  useEffect(() => {
    void refresh()
    const unsubscribe = window.prime.onEvent((event) => {
      if (!('type' in event) || event.type !== 'asr') return
      const payload = event.payload as AsrEvent
      if (payload.type === 'status') {
        setAsrStatus(payload.status)
      }
    })
    return unsubscribe
  }, [refresh])

  const pickLanguage = async (next: string) => {
    const normalized = normalizeAsrLanguage(next)
    setLanguage(normalized)
    setBusy(true)
    setError('')
    try {
      await call('sttSelect', { language: normalized })
    } catch (failure) {
      report(failure)
    } finally {
      setBusy(false)
    }
  }

  const retryWarmup = async () => {
    setError('')
    try {
      const next = (await window.prime.asr({ action: 'warmup' })) as AsrStatus | undefined
      if (next) setAsrStatus(next)
    } catch (failure) {
      report(failure)
    }
  }

  const statusText =
    asrStatus?.state === 'downloading'
      ? t('Загрузка модели распознавания речи · {value0}%', {
          value0: Math.floor((asrStatus.progress ?? 0) * 100)
        })
      : asrStatus?.state === 'loading'
        ? asrStatus.message || t('Загрузка модели…')
        : asrStatus?.state === 'error'
          ? asrStatus.message || t('Локальная модель распознавания речи недоступна')
          : asrStatus?.ready
            ? t('Модель готова (en-ru-uk)')
            : t('Подготовка модели…')

  return (
    <>
      <p className="muted">
        {t('Локальная модель en-ru-uk. Речь распознаётся на этом компьютере в реальном времени без отправки в интернет.')}
      </p>
      <p className="muted" role="status">
        {statusText}
      </p>
      {asrStatus?.state === 'error' && (
        <div className="settings-actions">
          <button type="button" onClick={() => void retryWarmup()}>
            <RefreshCw size={14} />
            {t('Повторить')}
          </button>
        </div>
      )}
      <label className="form-label">
        {t('Язык распознавания')}
        <Select
          label={t('Язык распознавания')}
          value={language}
          disabled={busy}
          onChange={(value) => void pickLanguage(value)}
          options={ASR_LANGUAGE_OPTIONS.map((item) => ({
            value: item.value,
            label: t(item.label)
          }))}
        />
      </label>
      {error && <p className="error-text">{error}</p>}
    </>
  )
}

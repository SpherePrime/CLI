import { useEffect, useRef, useState } from 'react'
import { Play, Trash2, Upload } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import type { SpeechStatus, SpeechVoice } from '../../../../shared/speech'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import { Speaker } from '../voice/speaker'
import '../../styles/conversation.css'

export function VoiceSettings({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [voices, setVoices] = useState<SpeechVoice[]>([])
  const [status, setStatus] = useState<SpeechStatus>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const speaker = useRef<Speaker>(undefined)
  const voice = w.preferences.speechVoice ?? 'F1'
  const speed = w.preferences.speechSpeed ?? 1.05
  const report = (failure: unknown) => setError(String((failure as Error)?.message || failure).replace(/^.*Error: /, ''))
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
      speaker.current ??= new Speaker({ voice, speed, onState: () => {}, onError: report })
      speaker.current.update({ voice, speed })
      await speaker.current.speak([t('Привет! Так я буду отвечать в голосовом разговоре.')])
    } finally {
      setBusy(false)
    }
  }
  const current = voices.find((item) => item.id === voice)
  return (
    <>
      <h3>{t('Голос')}</h3>
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
          options={voices.map((item) => ({ value: item.id, label: item.custom ? item.name : t(item.name) }))}
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
          onChange={(event) => void w.savePreferences({ speechSpeed: Number(event.target.value) }).catch(report)}
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

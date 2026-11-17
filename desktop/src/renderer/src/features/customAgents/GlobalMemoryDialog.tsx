import { useEffect, useRef, useState } from 'react'
import { BookText, Save, Sparkles, TriangleAlert, UserRound } from 'lucide-react'
import type { GlobalMemoryDoc } from '../../../../shared/customAgents'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'

type MemoryTab = 'memory' | 'user' | 'soul'

export function GlobalMemoryDialog({
  doc,
  onLoad,
  onSave,
  close
}: {
  doc: GlobalMemoryDoc | null
  onLoad: () => Promise<GlobalMemoryDoc>
  onSave: (input: Partial<GlobalMemoryDoc>) => Promise<GlobalMemoryDoc>
  close: () => void
}) {
  const { t } = useI18n()
  const [tab, setTab] = useState<MemoryTab>('memory')
  const [memory, setMemory] = useState(doc?.memory ?? '')
  const [userProfile, setUserProfile] = useState(doc?.user_profile ?? '')
  const [soul, setSoul] = useState(doc?.soul ?? '')
  const [loading, setLoading] = useState(!doc)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const initializedRef = useRef(Boolean(doc))
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const memoryLimit = doc?.memory_limit && doc.memory_limit > 0 ? doc.memory_limit : 2200
  const userLimit = doc?.user_limit && doc.user_limit > 0 ? doc.user_limit : 1375
  const memoryChars = Array.from(memory).length
  const userChars = Array.from(userProfile).length
  const memoryOver = memoryChars > memoryLimit
  const userOver = userChars > userLimit

  useEffect(() => {
    if (doc && !initializedRef.current) {
      setMemory(doc.memory)
      setUserProfile(doc.user_profile)
      setSoul(doc.soul)
      initializedRef.current = true
      setLoading(false)
    }
  }, [doc])

  useEffect(() => {
    if (doc) return
    let active = true
    setLoading(true)
    setError('')
    void onLoad()
      .then((loaded) => {
        if (!active) return
        setMemory(loaded.memory)
        setUserProfile(loaded.user_profile)
        setSoul(loaded.soul)
        initializedRef.current = true
      })
      .catch((cause) => {
        if (active) setError(String((cause as Error)?.message || cause))
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [doc, onLoad])

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (saving || loading) return
    if (memoryOver) {
      setTab('memory')
      setError(
        t('Превышен лимит MEMORY.md ({chars} / {limit} символов)', {
          chars: memoryChars,
          limit: memoryLimit
        })
      )
      return
    }
    if (userOver) {
      setTab('user')
      setError(
        t('Превышен лимит USER.md ({chars} / {limit} символов)', {
          chars: userChars,
          limit: userLimit
        })
      )
      return
    }
    setSaving(true)
    setError('')
    try {
      await onSave({
        memory,
        user_profile: userProfile,
        soul
      })
      close()
    } catch (cause) {
      setError(String((cause as Error)?.message || cause))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      title={t('Память и личность')}
      close={() => {
        if (!saving) close()
      }}
      wide
      className="ca-memory-modal"
      initialFocus={textareaRef}
    >
      <form className="ca-memory-form" onSubmit={handleSubmit}>
        <div className="ca-memory-tabs" role="tablist" aria-label={t('Файлы контекста')}>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'memory'}
            className={tab === 'memory' ? 'active' : ''}
            onClick={() => setTab('memory')}
          >
            <BookText size={14} aria-hidden="true" />
            <span>MEMORY.md</span>
            <span
              className={`ca-counter-pill ${memoryOver ? 'over' : memoryChars > memoryLimit * 0.85 ? 'warn' : ''}`}
            >
              {memoryChars} / {memoryLimit}
            </span>
          </button>

          <button
            type="button"
            role="tab"
            aria-selected={tab === 'user'}
            className={tab === 'user' ? 'active' : ''}
            onClick={() => setTab('user')}
          >
            <UserRound size={14} aria-hidden="true" />
            <span>USER.md</span>
            <span
              className={`ca-counter-pill ${userOver ? 'over' : userChars > userLimit * 0.85 ? 'warn' : ''}`}
            >
              {userChars} / {userLimit}
            </span>
          </button>

          <button
            type="button"
            role="tab"
            aria-selected={tab === 'soul'}
            className={tab === 'soul' ? 'active' : ''}
            onClick={() => setTab('soul')}
          >
            <Sparkles size={14} aria-hidden="true" />
            <span>SOUL.md</span>
            <span className="ca-counter-pill">{t('Опционально')}</span>
          </button>
        </div>

        {error && (
          <div className="ca-error-banner ca-memory-error" role="alert">
            <TriangleAlert size={15} aria-hidden="true" />
            <span>{error}</span>
          </div>
        )}

        {loading ? (
          <div className="ca-empty-compact">{t('Загрузка контекста…')}</div>
        ) : (
          <>
            {tab === 'memory' && (
              <label className="ca-field ca-memory-field">
                <div className="ca-memory-field-head">
                  <span>{t('Общая память (MEMORY.md) — факты среды и соглашения')}</span>
                  <span
                    className={`ca-counter-pill ${memoryOver ? 'over' : memoryChars > memoryLimit * 0.85 ? 'warn' : ''}`}
                  >
                    {memoryChars} / {memoryLimit}
                  </span>
                </div>
                <textarea
                  ref={textareaRef}
                  rows={11}
                  value={memory}
                  onChange={(event) => {
                    setMemory(event.target.value)
                    if (error) setError('')
                  }}
                  placeholder={t('Ключевые факты, правила проверки и договорённости…')}
                />
              </label>
            )}

            {tab === 'user' && (
              <label className="ca-field ca-memory-field">
                <div className="ca-memory-field-head">
                  <span>{t('Профиль пользователя (USER.md) — предпочтения и контекст')}</span>
                  <span
                    className={`ca-counter-pill ${userOver ? 'over' : userChars > userLimit * 0.85 ? 'warn' : ''}`}
                  >
                    {userChars} / {userLimit}
                  </span>
                </div>
                <textarea
                  ref={textareaRef}
                  rows={11}
                  value={userProfile}
                  onChange={(event) => {
                    setUserProfile(event.target.value)
                    if (error) setError('')
                  }}
                  placeholder={t('Язык, стиль ответов, стек и личные предпочтения…')}
                />
              </label>
            )}

            {tab === 'soul' && (
              <label className="ca-field ca-memory-field">
                <div className="ca-memory-field-head">
                  <span>{t('Базовая идентичность (SOUL.md) — опционально')}</span>
                  <span className="ca-counter-pill">{t('По умолчанию, если пусто')}</span>
                </div>
                <textarea
                  ref={textareaRef}
                  rows={11}
                  value={soul}
                  onChange={(event) => {
                    setSoul(event.target.value)
                    if (error) setError('')
                  }}
                  placeholder={t('Оставьте пустым для базового профиля или задайте тон и принципы…')}
                />
              </label>
            )}
          </>
        )}

        <div className="ca-modal-actions">
          <button type="button" onClick={close} disabled={saving}>
            {t('Отмена')}
          </button>
          <button type="submit" className="primary" disabled={saving || loading}>
            <Save size={14} aria-hidden="true" />
            <span>{saving ? t('Сохранение…') : t('Сохранить')}</span>
          </button>
        </div>
      </form>
    </Modal>
  )
}

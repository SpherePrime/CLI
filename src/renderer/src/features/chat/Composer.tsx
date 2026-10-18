import { PermissionMenu } from './PermissionMenu'
import { QueuePanel } from './QueuePanel'
import { ReasoningControl } from '../models/ReasoningControl'
import { ModelPicker } from '../models/ModelPicker'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import { CommandRunner } from './CommandRunner'
import { useEffect, useRef, useState } from 'react'
import {
  ArrowUp,
  ChevronDown,
  Mic,
  Paperclip,
  ShieldCheck,
  Square,
  X,
  ListOrdered,
  Sparkles,
  LoaderCircle
} from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import { useVoice } from '../plugins/useVoice'

export function Composer({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [text, setText] = useState('')
  const [modelsOpen, setModelsOpen] = useState(false)
  const modelAnchor = useRef<HTMLButtonElement>(null)
  const [attachments, setAttachments] = useState<any[]>([])
  const [sending, setSending] = useState(false)
  const [mode, setMode] = useState('general')
  const [skip, setSkip] = useState(false)
  const [suggestions, setSuggestions] = useState<any[]>([])
  const [command, setCommand] = useState<any>(null)
  const textarea = useRef<HTMLTextAreaElement>(null)
  useEffect(() => {
    const handler = (event: Event) => setCommand((event as CustomEvent).detail)
    window.addEventListener('prime:command', handler)
    const draft = (event: Event) => setText((event as CustomEvent).detail)
    window.addEventListener('prime:draft', draft)
    return () => {
      window.removeEventListener('prime:command', handler)
      window.removeEventListener('prime:draft', draft)
    }
  }, [])
  const key = `${!w.project || w.project.kind === 'chat' ? 'chat' : w.project.path}:${w.session?.id || 'new'}`
  const draftKey = useRef(key)
  const latest = useRef(text)
  latest.current = text
  const {
    recording,
    transcribing,
    toggle: toggleVoice
  } = useVoice(w, (transcript) => setText((value) => [value, transcript].filter(Boolean).join(' ')))
  useEffect(() => {
    if (draftKey.current !== key) {
      const previous = draftKey.current
      const draft = latest.current
      draftKey.current = key
      void w.savePreferences({ drafts: { [previous]: draft } })
      setText(w.preferences.drafts[key] || '')
      setAttachments([])
    }
  }, [key])
  useEffect(() => {
    const timer = setTimeout(() => {
      void w.savePreferences({ drafts: { [key]: text } })
    }, 600)
    return () => clearTimeout(timer)
  }, [text, key])
  useEffect(() => {
    if (w.project)
      void w
        .call('getSkip')
        .then((value) => setSkip(Boolean(value?.skip)))
        .catch(() => {})
  }, [w.project?.id])
  useEffect(() => {
    if (textarea.current) {
      textarea.current.style.height = 'auto'
      textarea.current.style.height = Math.min(220, textarea.current.scrollHeight) + 'px'
    }
  }, [text])
  async function submit() {
    if (!text.trim() || sending) return
    const submittedKey = key
    const submittedText = text
    setSending(true)
    if (
      await w.send(
        text,
        attachments.map((item) => item.id)
      )
    ) {
      await w.savePreferences({ drafts: { [submittedKey]: '' } })
      if (draftKey.current === submittedKey && latest.current === submittedText) {
        setText('')
        setAttachments([])
        setSuggestions([])
      }
    }
    setSending(false)
  }
  async function stop() {
    if (!w.activeSessionId) return
    try {
      await w.call('cancel', { sessionId: w.activeSessionId })
      w.setBusy(false)
      await w.refresh()
    } catch (error) {
      w.errorHandler(error)
    }
  }
  useEffect(() => {
    let lastEscape = 0
    const handler = (event: KeyboardEvent) => {
      if (document.querySelector('dialog[open]')) return
      if (event.key === 'Escape' && w.busy) {
        if (Date.now() - lastEscape < 600) void stop()
        lastEscape = Date.now()
      }
      if (event.altKey && event.code === 'KeyV') {
        event.preventDefault()
        void toggleVoice()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [w.busy, w.session?.id, recording])
  async function change(value: string) {
    setText(value)
    if (/@[^\s]*$/.test(value) && w.project && w.project.kind !== 'chat') {
      const query = value.split('@').pop()!.toLowerCase()
      const files = await window.prime.files(w.project.id).catch(() => [])
      setSuggestions(
        files
          .filter((file) => file.name.toLowerCase().includes(query))
          .slice(0, 8)
          .map((file) => ({
            label: '@' + file.path,
            description: file.directory ? t('Папка') : t('Файл'),
            file: file.path
          }))
      )
    } else setSuggestions([])
  }
  useEffect(() => {
    const open = () => setModelsOpen((value) => !value)
    window.addEventListener('prime:models', open)
    return () => window.removeEventListener('prime:models', open)
  }, [])
  const selectedModel = w.config?.models?.large
  return (
    <div className="composer-area">
      {modelsOpen && (
        <ModelPicker
          workspace={w}
          anchor={modelAnchor}
          close={() => setModelsOpen(false)}
          setup={() => {
            setModelsOpen(false)
            window.dispatchEvent(new Event('prime:providers'))
          }}
        />
      )}
      {command && (
        <CommandRunner
          command={command}
          workspace={w}
          close={() => setCommand(null)}
          onReady={setText}
        />
      )}
      <QueuePanel key={key} workspace={w} />
      {w.session?.todos?.length ? (
        <div className="todo-progress">
          {w.session.todos.filter((todo) => todo.status === 'completed').length} /{' '}
          {w.session.todos.length} {t('задач выполнено')}{' '}
          <span>{w.session.todos.find((todo) => todo.status === 'in_progress')?.active_form}</span>
        </div>
      ) : null}
      <div className={`composer ${recording ? 'recording' : ''}`}>
        {attachments.length > 0 && (
          <div className="attachment-chips">
            {attachments.map((item) => (
              <span key={item.id}>
                <Paperclip size={12} />
                {item.name}
                <button
                  className="icon small"
                  aria-label={t('Удалить вложение')}
                  onClick={() =>
                    setAttachments((items) => items.filter((file) => file.id !== item.id))
                  }
                >
                  <X size={12} />
                </button>
              </span>
            ))}
          </div>
        )}
        <textarea
          ref={textarea}
          value={text}
          onChange={(event) => void change(event.target.value)}
          placeholder={
            recording ? t('Запись голоса… Alt+V, чтобы остановить') : t('Напишите сообщение…')
          }
          aria-label={t('Сообщение Prime')}
          disabled={w.loading || sending}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
              event.preventDefault()
              void submit()
            }
          }}
        />
        {suggestions.length > 0 && (
          <div className="suggestions">
            {suggestions.map((item) => (
              <button
                key={item.label}
                onClick={async () => {
                  if (item.file) setText(text.replace(/@[^\s]*$/, `@${item.file} `))
                  setSuggestions([])
                }}
              >
                <strong>{item.label}</strong>
                <small>{item.description}</small>
              </button>
            ))}
          </div>
        )}
        <div className="composer-toolbar">
          <div className="composer-options">
            <button
              className="icon"
              title={t('Прикрепить файлы')}
              aria-label={t('Прикрепить файлы')}
              onClick={() =>
                void window.prime
                  .attachments()
                  .then((files) => setAttachments((items) => [...items, ...files]))
                  .catch(w.errorHandler)
              }
            >
              <Paperclip size={17} />
            </button>
            <span className="toolbar-divider" />
            <PermissionMenu
              skip={skip}
              disabled={!w.project}
              onChange={async (value) => {
                try {
                  await w.call('skip', { skip: value })
                  setSkip(value)
                } catch (error) {
                  w.errorHandler(error)
                }
              }}
            />
            <Select
              label={t('Режим агента')}
              value={mode}
              disabled={w.busy || !w.project}
              options={[
                { value: 'general', label: t('Работа') },
                { value: 'plan', label: t('План') }
              ]}
              onChange={(value) => {
                void w
                  .call('mode', { mode: value })
                  .then(() => setMode(value))
                  .catch(w.errorHandler)
              }}
            />
            <button
              ref={modelAnchor}
              className="model-trigger"
              aria-label={t('Выбрать модель')}
              aria-haspopup="listbox"
              aria-expanded={modelsOpen}
              onClick={() => setModelsOpen(!modelsOpen)}
              disabled={!w.project}
            >
              <Sparkles size={13} />
              <span>{selectedModel?.model || w.agent?.model?.name || t('Выбрать модель')}</span>
              <ChevronDown size={12} />
            </button>
            <ReasoningControl workspace={w} />
          </div>
          <div className="composer-actions">
            <button
              className={`icon ${recording ? 'danger' : ''}`}
              disabled={!w.project || transcribing}
              aria-label={recording ? t('Остановить запись') : t('Голосовой ввод')}
              title={t('Голосовой ввод · Alt V')}
              onClick={() => void toggleVoice()}
            >
              {transcribing ? <LoaderCircle size={17} className="spin" /> : <Mic size={17} />}
            </button>
            {w.busy && (!text.trim() || sending) ? (
              <button
                className="send stop"
                onClick={() => void stop()}
                aria-label={t('Остановить задачу')}
              >
                <Square size={14} fill="currentColor" />
              </button>
            ) : (
              <button
                className="send"
                aria-label={t('Отправить сообщение')}
                disabled={!text.trim() || sending || w.project?.connection !== 'ready'}
                onClick={() => void submit()}
              >
                {sending ? <LoaderCircle size={17} className="spin" /> : <ArrowUp size={18} />}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

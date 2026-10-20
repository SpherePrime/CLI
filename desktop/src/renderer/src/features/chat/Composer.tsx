import { PermissionMenu } from './PermissionMenu'
import { QueuePanel } from './QueuePanel'
import { ModelControl } from '../models/ModelControl'
import { VoiceWave } from './VoiceWave'
import { TodoPanel } from './TodoPanel'
import { Select } from '../../components/Select'
import { ProjectPicker } from './ProjectPicker'
import { useI18n } from '../../i18n'
import { CommandRunner } from './CommandRunner'
import { useEffect, useRef, useState } from 'react'
import { ArrowUp, Mic, Paperclip, Square, X, LoaderCircle } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import { useVoice } from '../plugins/useVoice'
import { useConversation } from '../voice/useConversation'
import { ConversationCall, ConversationCallPill } from '../voice/ConversationCall'
import { createPortal } from 'react-dom'
import { clipboardImages, imageUploads } from './pasteImages'
import { AudioLines } from 'lucide-react'
import '../../styles/attachments.css'

export function Composer({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [text, setText] = useState('')
  const [attachments, setAttachments] = useState<any[]>([])
  const inFlightAttachments = useRef<string[]>([])
  const attachmentRef = useRef(attachments)
  attachmentRef.current = attachments
  const [sending, setSending] = useState(false)
  const [importing, setImporting] = useState(false)
  const [callOpen, setCallOpen] = useState(false)
  const [callHost, setCallHost] = useState<Element | null>(null)
  const composerRoot = useRef<HTMLDivElement>(null)
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
  const conversation = useConversation(w)
  useEffect(() => {
    setCallHost(composerRoot.current?.closest('.main-workspace') || null)
  }, [])
  useEffect(() => {
    if (conversation.state === 'off') setCallOpen(false)
  }, [conversation.state])
  useEffect(() => {
    if (!callHost || !callOpen || conversation.state === 'off') return
    const hidden = [...callHost.children].filter(
      (element) => !element.classList.contains('voice-call')
    )
    const previous = hidden.map((element) => element.hasAttribute('inert'))
    hidden.forEach((element) => element.setAttribute('inert', ''))
    return () =>
      hidden.forEach((element, index) => {
        if (!previous[index]) element.removeAttribute('inert')
      })
  }, [callHost, callOpen, conversation.state === 'off'])
  useEffect(() => {
    if (callOpen && (w.permissions?.length || w.questions?.length)) setCallOpen(false)
  }, [callOpen, w.permissions?.length, w.questions?.length])
  useEffect(
    () =>
      window.prime.onEvent((event) => {
        if (
          event.workspaceId === w.project?.id &&
          event.type === 'windows' &&
          event.payload?.request
        )
          setCallOpen(false)
      }),
    [w.project?.id]
  )
  const draftKey = useRef(key)
  const latest = useRef(text)
  latest.current = text
  const submitting = useRef(false)
  const {
    recording,
    transcribing,
    analyser,
    toggle: toggleVoice,
    finish: finishVoice,
    cancel: cancelVoice
  } = useVoice(w, (transcript) => setText((value) => [value, transcript].filter(Boolean).join(' ')))
  useEffect(() => {
    if (draftKey.current !== key) {
      const previous = draftKey.current
      const draft = latest.current
      draftKey.current = key
      const justCreated =
        !w.preferences.drafts[key] &&
        !w.messages.length &&
        Date.now() / 1000 - (w.session?.updated_at || 0) < 5
      if (justCreated && draft && !submitting.current) return
      void w.savePreferences({ drafts: { [previous]: draft } })
      setText(w.preferences.drafts[key] || '')
      void window.prime
        .releaseAttachments(
          attachmentRef.current
            .map((item) => item.id)
            .filter((id) => !inFlightAttachments.current.includes(id))
        )
        .catch(w.errorHandler)
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
  const sessionKey = w.session?.id || ''
  useEffect(() => {
    if (!sessionKey || !w.project) return
    const target = w.preferences.skipBySession?.[sessionKey] ?? w.preferences.defaultSkip ?? false
    if (target === skip) return
    void w
      .call('skip', { skip: target })
      .then(() => setSkip(target))
      .catch(() => {})
  }, [sessionKey, w.project?.id, w.preferences.skipBySession, w.preferences.defaultSkip])
  useEffect(() => {
    if (textarea.current) {
      textarea.current.style.height = 'auto'
      textarea.current.style.height = Math.min(220, textarea.current.scrollHeight) + 'px'
    }
  }, [text])
  async function submit() {
    if (sending || importing || submitting.current) return
    submitting.current = true
    inFlightAttachments.current = attachments.map((item) => item.id)
    try {
      await deliver()
    } catch (error) {
      w.errorHandler(error)
    } finally {
      submitting.current = false
      const orphaned = inFlightAttachments.current.filter(
        (id) => draftKey.current !== key || !attachmentRef.current.some((item) => item.id === id)
      )
      inFlightAttachments.current = []
      if (orphaned.length) void window.prime.releaseAttachments(orphaned).catch(w.errorHandler)
      setSending(false)
    }
  }
  async function deliver() {
    const submittedKey = key
    let message = text
    if (recording) {
      setSending(true)
      const spoken = await finishVoice()
      message = [text, spoken].filter((part) => part.trim()).join(' ')
      setSending(false)
      if (draftKey.current !== submittedKey) return
    }
    if (!message.trim() && attachments.length) message = t('Посмотри на изображение.')
    if (!message.trim()) return
    const submittedText = message
    setSending(true)
    if (
      await w.send(
        message,
        attachments.map((item) => item.id)
      )
    ) {
      void window.prime.releaseAttachments(attachments.map((item) => item.id)).catch(w.errorHandler)
      await w.savePreferences({ drafts: { [submittedKey]: '' } })
      if (draftKey.current === submittedKey && submittedText.startsWith(latest.current)) {
        setText('')
        setAttachments([])
        setSuggestions([])
      }
    }
    setSending(false)
  }
  async function attach() {
    const targetKey = key
    setImporting(true)
    try {
      const added = await window.prime.attachments()
      if (draftKey.current === targetKey) setAttachments((items) => [...items, ...added])
      else await window.prime.releaseAttachments(added.map((item) => item.id))
    } catch (error) {
      w.errorHandler(error)
    } finally {
      setImporting(false)
    }
  }
  async function paste(event: React.ClipboardEvent<HTMLTextAreaElement>) {
    const images = clipboardImages(event.clipboardData)
    if (!images.length) return
    event.preventDefault()
    if (importing || sending) return
    const targetKey = key
    setImporting(true)
    try {
      const added = await window.prime.importImages(await imageUploads(images))
      if (draftKey.current === targetKey) setAttachments((items) => [...items, ...added])
      else await window.prime.releaseAttachments(added.map((item) => item.id))
    } catch (error) {
      w.errorHandler(error)
    } finally {
      setImporting(false)
    }
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
      if (event.altKey && event.code === 'KeyV' && conversation.state === 'off') {
        event.preventDefault()
        void toggleVoice()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [w.busy, w.session?.id, recording, conversation.state])
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
  const voiceActive = recording || (transcribing && sending)
  return (
    <div className="composer-area" ref={composerRoot}>
      {callHost &&
        callOpen &&
        conversation.state !== 'off' &&
        createPortal(
          <ConversationCall conversation={conversation} onOpenChat={() => setCallOpen(false)} />,
          callHost
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
      {w.session?.todos?.length ? <TodoPanel todos={w.session.todos} /> : null}
      {conversation.state !== 'off' && !callOpen && (
        <ConversationCallPill
          conversation={conversation}
          onReturnToCall={() => setCallOpen(true)}
        />
      )}
      <div className={`composer ${recording ? 'recording' : ''}`}>
        {attachments.length > 0 && (
          <div className="attachment-chips">
            {attachments.map((item) => (
              <span key={item.id} className={item.preview ? 'attachment-image' : ''}>
                {item.preview ? (
                  <img src={item.preview} alt={item.name} />
                ) : (
                  <Paperclip size={12} />
                )}
                {item.name}
                <button
                  className="icon small"
                  aria-label={t('Удалить вложение')}
                  disabled={sending || importing}
                  onClick={() => {
                    setAttachments((items) => items.filter((file) => file.id !== item.id))
                    void window.prime.releaseAttachments([item.id]).catch(w.errorHandler)
                  }}
                >
                  <X size={12} />
                </button>
              </span>
            ))}
          </div>
        )}
        {importing && (
          <div className="attachment-import" role="status">
            <LoaderCircle size={14} className="spin" />
            {t('Добавление изображения…')}
          </div>
        )}
        {voiceActive && (
          <VoiceWave
            analyser={analyser}
            transcribing={transcribing}
            onCancel={() => void cancelVoice()}
          />
        )}
        <textarea
          hidden={voiceActive}
          ref={textarea}
          value={text}
          onChange={(event) => void change(event.target.value)}
          onPaste={(event) => void paste(event)}
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
              disabled={sending || importing}
              onClick={() => void attach()}
            >
              <Paperclip size={17} />
            </button>
            <span className="toolbar-divider" />
            <ProjectPicker workspace={w} />
            <PermissionMenu
              skip={skip}
              disabled={!w.project}
              onChange={async (value) => {
                try {
                  await w.call('skip', { skip: value })
                  setSkip(value)
                  void w.savePreferences({
                    defaultSkip: value,
                    ...(sessionKey ? { skipBySession: { [sessionKey]: value } } : {})
                  })
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
          </div>
          <div className="composer-actions">
            <ModelControl workspace={w} />
            <button
              className={`icon ${conversation.state !== 'off' ? 'active' : ''}`}
              disabled={!w.project || recording}
              aria-label={
                conversation.state !== 'off' ? t('Вернуться к разговору') : t('Голосовой разговор')
              }
              aria-pressed={conversation.state !== 'off'}
              title={t('Голосовой разговор')}
              onClick={() => {
                setCallOpen(true)
                if (conversation.state === 'off') void conversation.start()
              }}
            >
              <AudioLines size={17} />
            </button>
            <button
              className={`icon ${recording ? 'danger' : ''}`}
              disabled={!w.project || transcribing || conversation.state !== 'off'}
              aria-label={recording ? t('Остановить запись') : t('Голосовой ввод')}
              title={t('Голосовой ввод · Alt V')}
              onClick={() => void toggleVoice()}
            >
              {transcribing ? <LoaderCircle size={17} className="spin" /> : <Mic size={17} />}
            </button>
            {w.busy && !recording && ((!text.trim() && !attachments.length) || sending) ? (
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
                disabled={
                  (!text.trim() && !recording && !attachments.length) ||
                  sending ||
                  importing ||
                  w.project?.connection !== 'ready'
                }
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

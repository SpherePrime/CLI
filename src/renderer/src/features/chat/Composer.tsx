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

export function Composer({
  workspace: w,
  onModels
}: {
  workspace: Workspace
  onModels: () => void
}) {
  const [text, setText] = useState('')
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
  const key = `${w.project?.path || ''}:${w.session?.id || 'new'}`
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
      if (w.project) void w.savePreferences({ drafts: { [key]: text } })
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
    if (value.startsWith('/') && !value.includes(' ')) {
      const builtIn = [
        { label: '/compact', description: 'Сжать контекст', action: 'compact' },
        { label: '/plan', description: 'Режим планирования', action: 'plan' },
        { label: '/code', description: 'Режим работы', action: 'general' }
      ]
      try {
        const lists = await Promise.all([
          w.call('commands').catch(() => []),
          w.call('mcpPrompts').catch(() => [])
        ])
        const custom = lists.flat()
        const items = Array.isArray(custom)
          ? custom.map((item: any) => ({
              label: '/' + (item.Name || item.name || item.title || item.id),
              description: item.description,
              prompt: item.Content || item.prompt || item.content,
              command: item
            }))
          : []
        setSuggestions([...builtIn, ...items].filter((item) => item.label.startsWith(value)))
      } catch {
        setSuggestions(builtIn.filter((item) => item.label.startsWith(value)))
      }
    } else if (/@[^\s]*$/.test(value) && w.project) {
      const query = value.split('@').pop()!.toLowerCase()
      const files = await window.prime.files(w.project.id).catch(() => [])
      setSuggestions(
        files
          .filter((file) => file.name.toLowerCase().includes(query))
          .slice(0, 8)
          .map((file) => ({
            label: '@' + file.path,
            description: file.directory ? 'Папка' : 'Файл',
            file: file.path
          }))
      )
    } else setSuggestions([])
  }
  const selectedModel = w.config?.models?.large
  return (
    <div className="composer-area">
      {command && (
        <CommandRunner
          command={command}
          workspace={w}
          close={() => setCommand(null)}
          onReady={setText}
        />
      )}
      {w.queue.length > 0 && (
        <details className="queue">
          <summary>
            <ListOrdered size={14} /> В очереди: {w.queue.length}
          </summary>
          {w.queue.map((prompt, i) => (
            <p key={i}>{typeof prompt === 'string' ? prompt : JSON.stringify(prompt)}</p>
          ))}
          <button
            className="quiet"
            onClick={() =>
              void w
                .call('clearQueue', { sessionId: w.session?.id })
                .then(w.refresh)
                .catch(w.errorHandler)
            }
          >
            Очистить очередь
          </button>
        </details>
      )}
      {w.session?.todos?.length ? (
        <div className="todo-progress">
          {w.session.todos.filter((todo) => todo.status === 'completed').length} /{' '}
          {w.session.todos.length} задач выполнено{' '}
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
                  aria-label="Удалить вложение"
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
            recording
              ? 'Запись голоса… Alt+V, чтобы остановить'
              : w.project
                ? 'Опишите задачу…  @ файлы, / команды'
                : 'Откройте проект, чтобы начать'
          }
          aria-label="Сообщение Prime"
          disabled={!w.project || w.loading || sending}
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
                  if (item.action === 'compact') {
                    await w.call('summarize', { sessionId: w.session?.id }).catch(w.errorHandler)
                    setText('')
                  } else if (item.action) {
                    await w.call('mode', { mode: item.action }).catch(w.errorHandler)
                    setMode(item.action)
                    setText('')
                  } else if (item.file) setText(text.replace(/@[^\s]*$/, `@${item.file} `))
                  else if (item.command) setCommand(item.command)
                  else setText(item.prompt || item.label + ' ')
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
              disabled={!w.project}
              title="Прикрепить файлы"
              aria-label="Прикрепить файлы"
              onClick={() =>
                void window.prime
                  .attachments()
                  .then((files) => setAttachments((items) => [...items, ...files]))
                  .catch(w.errorHandler)
              }
            >
              <Paperclip size={17} />
            </button>
            <button
              className={`icon ${recording ? 'danger' : ''}`}
              disabled={!w.project || transcribing}
              aria-label={recording ? 'Остановить запись' : 'Голосовой ввод'}
              title="Голосовой ввод · Alt V"
              onClick={() => void toggleVoice()}
            >
              {transcribing ? <LoaderCircle size={17} className="spin" /> : <Mic size={17} />}
            </button>
            <span className="toolbar-divider" />
            <select
              aria-label="Режим агента"
              value={mode}
              disabled={w.busy || !w.project}
              onChange={async (event) => {
                const value = event.target.value
                try {
                  await w.call('mode', { mode: value })
                  setMode(value)
                } catch (error) {
                  w.errorHandler(error)
                }
              }}
            >
              <option value="general">Работа</option>
              <option value="plan">План</option>
            </select>
            <button className="model-trigger" onClick={onModels} disabled={!w.project}>
              <Sparkles size={13} />
              <span>{selectedModel?.model || w.agent?.model?.name || 'Выбрать модель'}</span>
              <ChevronDown size={12} />
            </button>
          </div>
          {w.busy && (!text.trim() || sending) ? (
            <button
              className="send stop"
              onClick={() => void stop()}
              aria-label="Остановить задачу"
            >
              <Square size={14} fill="currentColor" />
            </button>
          ) : (
            <button
              className="send"
              aria-label="Отправить сообщение"
              disabled={!text.trim() || sending || w.project?.connection !== 'ready'}
              onClick={() => void submit()}
            >
              {sending ? <LoaderCircle size={17} className="spin" /> : <ArrowUp size={18} />}
            </button>
          )}
        </div>
      </div>
      <div className="composer-meta">
        <button
          onClick={async () => {
            try {
              await w.call('skip', { skip: !skip })
              setSkip(!skip)
            } catch (error) {
              w.errorHandler(error)
            }
          }}
          disabled={!w.project}
        >
          <ShieldCheck size={12} />
          {skip ? 'YOLO · без подтверждений' : 'С подтверждением действий'}
          <ChevronDown size={10} />
        </button>
        <span>
          Shift Enter — новая строка
          {selectedModel?.reasoning_mode ? ` · ${selectedModel.reasoning_mode}` : ''}
        </span>
      </div>
    </div>
  )
}

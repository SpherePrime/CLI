import { useState } from 'react'
import { Check, Eye, FileCode2, Plus, Target, X } from 'lucide-react'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'
import {
  buildAgentPrompt,
  emptyAgentContext,
  mergeContextSource
} from '../../../../shared/agentContext'
import type { Workspace } from '../../app/useWorkspace'

export function ContextControl({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const context = w.agentContext || emptyAgentContext()
  const hasContext = Boolean(
    context.goal || context.constraints || context.criteria || context.sources.length
  )
  return (
    <>
      <button
        className={`context-control ${hasContext && context.enabled ? 'enabled' : ''}`}
        disabled={!w.project}
        onClick={() => setOpen(true)}
        aria-label={t('Задача и контекст')}
      >
        <Target size={14} />
        <span>{t('Задача и контекст')}</span>
        {hasContext && (
          <small>
            {context.enabled ? <Check size={12} /> : t('Выкл.')}
            {context.sources.length || ''}
          </small>
        )}
        <Plus size={13} />
      </button>
      {open && <ContextDialog key={w.project?.id} workspace={w} close={() => setOpen(false)} />}
    </>
  )
}

function ContextDialog({ workspace: w, close }: { workspace: Workspace; close: () => void }) {
  const { t } = useI18n()
  const [context, setContext] = useState(() => {
    const saved = w.agentContext || emptyAgentContext()
    return {
      ...saved,
      enabled: w.preferences.agentContexts?.[w.project?.path || ''] ? saved.enabled : true
    }
  })
  const [path, setPath] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [preview, setPreview] = useState('')
  async function attempt(action: () => Promise<void>) {
    setBusy(true)
    setError('')
    try {
      await action()
    } catch (error) {
      setError(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal title={t('Задача и контекст')} close={close} className="context-dialog" wide>
      <p className="muted">
        {t(
          'Цель, ограничения и выбранные файлы передаются агенту с каждым сообщением в этом проекте.'
        )}
      </p>
      <div className="context-fields">
        <label>
          {t('Цель задачи')}
          <textarea
            rows={2}
            maxLength={4000}
            value={context.goal}
            onChange={(event) => setContext({ ...context, goal: event.target.value })}
            placeholder={t('Какой результат нужен?')}
          />
        </label>
        <label>
          {t('Ограничения')}
          <textarea
            rows={2}
            maxLength={4000}
            value={context.constraints}
            onChange={(event) => setContext({ ...context, constraints: event.target.value })}
            placeholder={t('Что нужно сохранить и чего избегать?')}
          />
        </label>
        <label>
          {t('Критерии готовности')}
          <textarea
            rows={2}
            maxLength={4000}
            value={context.criteria}
            onChange={(event) => setContext({ ...context, criteria: event.target.value })}
            placeholder={t('Как проверить, что задача выполнена?')}
          />
        </label>
      </div>
      <div className="context-source-heading">
        <strong>{t('Файлы и фрагменты')}</strong>
        <span>{context.sources.length} / 8</span>
      </div>
      <p className="muted small-text">
        {t(
          'Файлы перечитываются перед отправкой. Фрагменты сохраняют выбранный текст, включая несохранённые правки.'
        )}
      </p>
      <div className="context-sources">
        {context.sources.map((source) => (
          <div key={source.id}>
            <FileCode2 size={15} />
            <span title={source.path}>
              {source.path}
              {source.kind === 'selection' && `:${source.startLine}–${source.endLine}`}
            </span>
            <small>{source.kind === 'file' ? t('Файл') : t('Фрагмент')}</small>
            <button
              className="icon small"
              aria-label={t('Убрать из контекста {path}', { path: source.path })}
              onClick={() => {
                setContext({
                  ...context,
                  sources: context.sources.filter((item) => item.id !== source.id)
                })
                setPreview('')
              }}
            >
              <X size={13} />
            </button>
          </div>
        ))}
      </div>
      {w.project?.kind !== 'chat' && (
        <div className="context-path">
          <input
            aria-label={t('Путь файла для контекста')}
            placeholder="src/app.ts"
            value={path}
            onChange={(event) => setPath(event.target.value)}
          />
          <button
            disabled={busy || !path.trim() || context.sources.length >= 8}
            onClick={() =>
              void attempt(async () => {
                const relative = path.trim()
                const file = await window.prime.readFile(w.project!.id, relative)
                if (file.image || typeof file.content !== 'string')
                  throw new Error(t('Выберите текстовый файл'))
                setContext((current) => ({
                  ...mergeContextSource(current, {
                    id: `file:${relative}`,
                    kind: 'file',
                    path: relative
                  }),
                  enabled: current.enabled
                }))
                setPath('')
                setPreview('')
              })
            }
          >
            <Plus size={14} />
            {t('Добавить')}
          </button>
        </div>
      )}
      <label className="checkbox-label">
        <input
          type="checkbox"
          checked={context.enabled}
          onChange={(event) => setContext({ ...context, enabled: event.target.checked })}
        />
        {t('Подключать к сообщениям')}
      </label>
      {error && (
        <p role="alert" className="danger">
          {error}
        </p>
      )}
      {preview && <pre className="context-preview">{preview}</pre>}
      <footer className="context-dialog-actions">
        <button
          disabled={busy}
          onClick={() =>
            void attempt(async () =>
              setPreview(
                await buildAgentPrompt('', context, (file) =>
                  window.prime.readFile(w.project!.id, file)
                )
              )
            )
          }
        >
          <Eye size={15} />
          {t('Посмотреть контекст')}
        </button>
        <button
          disabled={busy}
          className="primary"
          onClick={() =>
            void attempt(async () => {
              await w.updateAgentContext(context)
              close()
            })
          }
        >
          <Check size={15} />
          {t('Сохранить')}
        </button>
      </footer>
    </Modal>
  )
}

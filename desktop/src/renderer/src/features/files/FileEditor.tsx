import { useEffect, useState } from 'react'
import { CodeEditor, modelUri } from './CodeEditor'
import { useRef } from 'react'
import { X, Save, FileCode2, RefreshCw, Columns2 } from 'lucide-react'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'
import { monaco, languageFor } from './monaco'
import type { Documents } from './useDocuments'

export default function FileEditor({ state, theme }: { state: Documents; theme: string }) {
  const { t } = useI18n()
  const opened = useRef<string[]>([])
  const [closing, setClosing] = useState('')
  const [reloading, setReloading] = useState(false)
  const [position, setPosition] = useState({ line: 1, column: 1 })
  const [editorTheme, setEditorTheme] = useState('prime-dark')
  const document = state.documents.find((item) => item.id === state.active)
  useEffect(() => {
    const media = matchMedia('(prefers-color-scheme:dark)')
    const update = () =>
      setEditorTheme(
        theme === 'light' || (theme === 'system' && !media.matches) ? 'prime-light' : 'prime-dark'
      )
    update()
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [theme])
  useEffect(() => {
    const save = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.code === 'KeyS') {
        event.preventDefault()
        event.stopPropagation()
        if (document) void state.save(document.id)
      }
    }
    window.addEventListener('keydown', save, true)
    return () => window.removeEventListener('keydown', save, true)
  }, [document, state.save])
  useEffect(() => {
    const ids = state.documents
      .filter((item) => !item.image && item.original === undefined)
      .map((item) => modelUri(item.workspaceId, item.path).toString())
    for (const uri of opened.current)
      if (!ids.includes(uri)) monaco.editor.getModel(monaco.Uri.parse(uri))?.dispose()
    opened.current = ids
  }, [state.documents])
  useEffect(
    () => () => {
      for (const uri of opened.current) monaco.editor.getModel(monaco.Uri.parse(uri))?.dispose()
    },
    []
  )
  if (!document) return null
  const dirty = document.content !== document.savedContent
  const readOnly = Boolean(document.image) || document.original !== undefined
  return (
    <section className="file-editor" aria-label={t('Редактор файлов')}>
      <div className="editor-tabs" role="tablist" aria-label={t('Открытые файлы')}>
        {state.documents.map((item) => (
          <div className={`editor-tab ${item.id === document.id ? 'active' : ''}`} key={item.id}>
            <button
              role="tab"
              aria-selected={item.id === document.id}
              title={item.path}
              onClick={() => state.setActive(item.id)}
            >
              <FileCode2 size={14} />
              <span>{item.name}</span>
              {item.source && <small>{item.source}</small>}
              {item.content !== item.savedContent && <span className="unsaved-dot" />}
            </button>
            <button
              aria-label={t('Закрыть файл {name}', { name: item.name })}
              className="icon small"
              onClick={() =>
                item.content !== item.savedContent ? setClosing(item.id) : state.close(item.id)
              }
            >
              <X size={13} />
            </button>
          </div>
        ))}
      </div>
      <div className="editor-toolbar">
        <span title={document.path}>{document.path}</span>
        {!readOnly && (
          <>
            <button
              className="icon small"
              title={t('Перечитать файл')}
              aria-label={t('Перечитать файл')}
              onClick={() => (dirty ? setReloading(true) : void state.reload(document.id))}
            >
              <RefreshCw size={14} />
            </button>
            <button
              className="editor-save"
              disabled={!dirty || state.saving}
              onClick={() => void state.save(document.id)}
            >
              <Save size={14} />
              {t('Сохранить')}
            </button>
          </>
        )}
      </div>
      <div className="editor-surface">
        {document.image ? (
          <div className="editor-image">
            <img src={document.image} alt={document.name} />
          </div>
        ) : (
          <CodeEditor
            workspaceId={document.workspaceId}
            path={document.path}
            value={document.content}
            original={document.original}
            language={languageFor(document.path)}
            theme={editorTheme}
            label={t('Код файла')}
            onChange={(value) => state.edit(document.id, value)}
            onPosition={setPosition}
          />
        )}
      </div>
      <div className="editor-status">
        <span>{readOnly ? t('Только чтение') : dirty ? t('Не сохранено') : t('Сохранено')}</span>
        <span>
          {languageFor(document.path)} · UTF-8
          {!readOnly && ` · ${position.line}:${position.column}`}
        </span>
      </div>
      {closing && (
        <Modal title={t('Сохранить изменения файла?')} close={() => setClosing('')}>
          <p>{state.documents.find((item) => item.id === closing)?.path}</p>
          <div className="approval-actions">
            <button onClick={() => setClosing('')}>{t('Отмена')}</button>
            <button
              onClick={() => {
                state.close(closing)
                setClosing('')
              }}
            >
              {t('Не сохранять')}
            </button>
            <button
              className="primary"
              onClick={async () => {
                if (await state.save(closing)) {
                  if (state.closeSaved(closing)) setClosing('')
                }
              }}
            >
              {t('Сохранить')}
            </button>
          </div>
        </Modal>
      )}
      {reloading && (
        <Modal title={t('Перечитать файл с диска?')} close={() => setReloading(false)}>
          <p>{t('Несохранённые изменения этой вкладки будут заменены содержимым файла.')}</p>
          <div className="approval-actions">
            <button onClick={() => setReloading(false)}>{t('Отмена')}</button>
            <button
              onClick={() => {
                void state.reload(document.id)
                setReloading(false)
              }}
            >
              {t('Перечитать')}
            </button>
          </div>
        </Modal>
      )}
    </section>
  )
}

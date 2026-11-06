import { useEffect, useRef, useState } from 'react'
import { FolderOpen, FolderPlus, LoaderCircle } from 'lucide-react'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import { projectCreationPath, projectNameError } from '../../../../shared/projectCreation'
import '../../styles/project-creation.css'

export function CreateProjectDialog({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [name, setName] = useState(t('Новый проект'))
  const [base, setBase] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const submitting = useRef(false)
  const [previousFocus] = useState(() => typeof document === 'undefined' ? null : document.activeElement)
  useEffect(() => () => {
    queueMicrotask(() => {
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected) previousFocus.focus()
    })
  }, [previousFocus])
  useEffect(() => {
    let active = true
    window.prime.projectCreationBase().then((value) => {
      if (active) setBase(value)
    }).catch(() => {
      if (active) setError(t('Не удалось определить папку «Документы»'))
    })
    input.current?.select()
    return () => { active = false }
  }, [])
  const invalid = projectNameError(name)
  const close = () => { if (!submitting.current) w.closeProjectCreation() }
  async function create() {
    if (!base || invalid || submitting.current) return
    submitting.current = true
    setPending(true)
    setError('')
    try {
      await w.createProject(name)
      w.closeProjectCreation()
    } catch (cause) {
      const message = String((cause as Error)?.message || cause)
      setError(t(message === 'PROJECT_EXISTS' ? 'Папка уже существует. Выберите другое имя или откройте папку.'
        : message === 'INVALID_PROJECT_NAME' ? 'Введите корректное имя: до 80 символов, без служебных имён и символов пути'
        : message === 'PROJECT_ACCESS_DENIED' ? 'Нет доступа для создания папки'
        : 'Не удалось создать или открыть проект. Если папка создана, откройте её через «Открыть папку».'))
    } finally {
      submitting.current = false
      setPending(false)
    }
  }
  return (
    <Modal title={t('Новый проект')} close={close} className="project-creation-modal" initialFocus={input}>
      <form className="project-creation" aria-busy={pending} onSubmit={(event) => { event.preventDefault(); void create() }}>
        <div className="project-creation-intro">
          <div className="project-creation-symbol"><FolderPlus size={26} /></div>
          <div><h3>{t('Место для ваших идей')}</h3><p>{t('Создадим новую папку и откроем её в Prime.')}</p></div>
        </div>
        <label htmlFor="project-creation-name">{t('Название проекта')}</label>
        <input ref={input} id="project-creation-name" value={name} disabled={pending} autoComplete="off" spellCheck={false}
          aria-invalid={Boolean(invalid)} aria-describedby="project-creation-validation project-creation-path"
          onChange={(event) => { setName(event.target.value); setError('') }} />
        <div id="project-creation-validation" className="project-creation-validation" aria-live="polite">{invalid ? t(invalid) : t('Отдельная папка. Существующие файлы останутся нетронутыми.')}</div>
        <div className="project-creation-location"><span>{t('Расположение')}</span><output id="project-creation-path" aria-live="polite">{base ? projectCreationPath(base, name) : t('Определяем папку «Документы»…')}</output></div>
        {error && <p className="project-creation-error" role="alert">{error}</p>}
        <div className="project-creation-actions">
          <button type="button" disabled={pending} onClick={async () => { w.closeProjectCreation(); await w.openFolder() }}><FolderOpen size={16} />{t('Открыть папку')}</button>
          <button type="submit" className="primary" disabled={pending || !base || Boolean(invalid)}>{pending ? <LoaderCircle size={16} className="spin" /> : <FolderPlus size={16} />}{t(pending ? 'Создаём проект…' : 'Создать проект')}</button>
        </div>
      </form>
    </Modal>
  )
}

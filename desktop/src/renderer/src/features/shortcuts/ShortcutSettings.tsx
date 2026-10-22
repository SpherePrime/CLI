import { useEffect, useState } from 'react'
import { RotateCcw } from 'lucide-react'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import {
  defaultShortcuts,
  mouseShortcutFromEvent,
  shortcutActions,
  shortcutConflicts,
  shortcutFromEvent,
  shortcutLabel,
  type ShortcutAction
} from '../../../../shared/keybindings'
import { useShortcuts } from './shortcutStore'
import '../../styles/shortcuts.css'

export function ShortcutSettings({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const shortcuts = useShortcuts()
  const [recording, setRecording] = useState<ShortcutAction | null>(null)
  const labels: Record<ShortcutAction, string> = {
    commands: t('Быстрые действия'),
    chatSearch: t('Поиск в чате'),
    focus: t('Режим фокуса'),
    settings: t('Открыть настройки'),
    newChat: t('Новый чат'),
    models: t('Выбрать модель'),
    terminal: t('Терминал проекта'),
    voice: t('Голосовой ввод'),
    saveFile: t('Сохранить файл'),
    zoomIn: t('Увеличить масштаб'),
    zoomOut: t('Уменьшить масштаб'),
    zoomReset: t('Сбросить масштаб')
  }
  const conflicts = shortcutConflicts(shortcuts)
  function save(next: Partial<Record<ShortcutAction, string>>) {
    const custom: Record<string, string> = {}
    const merged = { ...shortcuts, ...next }
    for (const action of shortcutActions)
      if (merged[action] !== defaultShortcuts[action]) custom[action] = merged[action]
    void w.savePreferences({ keybindings: custom }).catch(w.errorHandler)
  }
  useEffect(() => {
    if (!recording) return
    const capture = (event: KeyboardEvent) => {
      event.preventDefault()
      event.stopImmediatePropagation()
      const plain = !event.ctrlKey && !event.altKey && !event.shiftKey && !event.metaKey
      if (plain && event.key === 'Escape') return setRecording(null)
      if (plain && (event.key === 'Backspace' || event.key === 'Delete')) {
        save({ [recording]: '' })
        return setRecording(null)
      }
      const combo = shortcutFromEvent(event)
      if (!combo) return
      save({ [recording]: combo })
      setRecording(null)
    }
    // Mouse buttons: side (back/forward) buttons only fire pointer events,
    // so a keyboard-only recorder could never see them. While recording,
    // pressing the back/forward/middle button (or any click with a modifier)
    // becomes the new binding; a plain left click keeps meaning "cancel".
    const handlePointer = (event: PointerEvent) => {
      if ((event.target as Element).closest?.('.shortcut-key.recording')) {
        setRecording(null)
        return
      }
      const modified = event.ctrlKey || event.altKey || event.shiftKey || event.metaKey
      if (event.button === 0 && !modified) {
        setRecording(null)
        return
      }
      const code = mouseShortcutFromEvent(event)
      if (!code) return
      event.preventDefault()
      save({ [recording]: code })
      setRecording(null)
    }
    window.addEventListener('keydown', capture, true)
    window.addEventListener('pointerdown', handlePointer, true)
    return () => {
      window.removeEventListener('keydown', capture, true)
      window.removeEventListener('pointerdown', handlePointer, true)
    }
  }, [recording, shortcuts])
  const customized = shortcutActions.some((action) => shortcuts[action] !== defaultShortcuts[action])
  return (
    <section className="shortcut-settings" aria-label={t('Горячие клавиши')}>
      <header>
        <div>
          <h3>{t('Горячие клавиши')}</h3>
          <small>
            {t('Нажмите на сочетание и введите новое. Esc — отмена, Backspace — отключить. Для боковых кнопок мыши нажмите их во время записи.')}
          </small>
        </div>
        <button disabled={!customized} onClick={() => void w.savePreferences({ keybindings: {} })}>
          <RotateCcw size={14} />
          {t('Сбросить все')}
        </button>
      </header>
      <ul>
        {shortcutActions.map((action) => {
          const conflict = conflicts.get(action)
          const active = recording === action
          return (
            <li key={action} className={conflict ? 'conflict' : ''}>
              <span className="shortcut-name">
                {labels[action]}
                {conflict && (
                  <small>
                    {t('Совпадает с: {value}', {
                      value: conflict.map((other) => labels[other]).join(', ')
                    })}
                  </small>
                )}
              </span>
              <button
                className={`shortcut-key ${active ? 'recording' : ''}`}
                aria-label={t('Изменить сочетание: {value}', { value: labels[action] })}
                onClick={() => setRecording(active ? null : action)}
              >
                {active
                  ? t('Нажмите клавишу или кнопку мыши…')
                  : shortcuts[action]
                    ? shortcutLabel(shortcuts[action])
                    : t('Не задано')}
              </button>
              <button
                className="icon small"
                title={t('Сбросить')}
                aria-label={t('Сбросить')}
                disabled={shortcuts[action] === defaultShortcuts[action]}
                onClick={() => save({ [action]: defaultShortcuts[action] })}
              >
                <RotateCcw size={13} />
              </button>
            </li>
          )
        })}
        {[
          [t('Отправить сообщение'), 'Enter'],
          [t('Новая строка'), 'Shift Enter'],
          [t('Остановить задачу'), 'Esc Esc']
        ].map(([label, key]) => (
          <li key={label} className="fixed">
            <span className="shortcut-name">{label}</span>
            <kbd className="shortcut-key">{key}</kbd>
            <span className="icon small" />
          </li>
        ))}
      </ul>
    </section>
  )
}

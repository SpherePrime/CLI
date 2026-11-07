import { useRef, useState } from 'react'
import { Target, Check, Pencil, Play, X } from 'lucide-react'
import { Popover } from '../../components/Popover'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'
import { useChatGoal } from './useChatGoal'
import { GoalEditor } from './GoalEditor'
import { GoalStatus } from './GoalStatus'
import { GoalRing } from './GoalRing'
import './goal.css'

export function GoalButton({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const anchor = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(false)
  const chatGoal = useChatGoal(w)
  const { goal, progress, phase, saving, busy, hasSession, stages } = chatGoal
  const showEditor = editing || !goal
  const close = () => {
    setOpen(false)
    setEditing(false)
  }
  const canResume =
    phase === 'paused' && (progress.total === 0 || progress.completed < progress.total)
  return (
    <>
      <button
        ref={anchor}
        className={`goal-chip ${goal ? `has-goal ${phase}` : ''}`}
        title={goal ? goal.text : t('Задать цель чата')}
        aria-label={t('Цель чата')}
        aria-expanded={open}
        disabled={!w.project}
        onClick={() => (open ? close() : setOpen(true))}
      >
        {!goal ? (
          <Target size={14} />
        ) : phase === 'achieved' ? (
          <Check size={14} strokeWidth={2.6} />
        ) : (
          <GoalRing ratio={progress.ratio} size={16} />
        )}
        <span className="goal-chip-label">{t('Цель')}</span>
        {goal && phase !== 'achieved' && progress.total > 0 && (
          <span className="goal-chip-count">
            {progress.completed}/{progress.total}
          </span>
        )}
      </button>
      {open && (
        <Popover anchor={anchor} close={close} width={380} label={t('Цель чата')}>
          <div className="goal-panel">
            <header className="goal-panel-head">
              <span className="goal-panel-icon">
                <Target size={15} />
              </span>
              <div>
                <strong>{t('Цель чата')}</strong>
                <small>
                  {t('Агент доводит цель до готового проверенного результата без остановок.')}
                </small>
              </div>
            </header>
            {showEditor ? (
              <GoalEditor
                initial={goal?.text || ''}
                canSave={hasSession}
                canStart={!busy}
                saving={saving}
                onCancel={goal ? () => setEditing(false) : undefined}
                onSave={async (text, start) => {
                  if (await chatGoal.save(text, start)) {
                    setEditing(false)
                    if (start) close()
                  }
                }}
              />
            ) : (
              goal && (
                <>
                  <GoalStatus goal={goal} phase={phase} progress={progress} stages={stages} />
                  <footer className="goal-actions">
                    <button className="quiet danger" onClick={() => void chatGoal.clear()}>
                      <X size={14} />
                      {t('Снять')}
                    </button>
                    <button className="quiet" onClick={() => setEditing(true)}>
                      <Pencil size={13} />
                      {t('Изменить')}
                    </button>
                    {canResume && (
                      <button
                        className="primary"
                        onClick={() => {
                          close()
                          void chatGoal.resume(t('Продолжи'))
                        }}
                      >
                        <Play size={13} />
                        {t('Продолжить')}
                      </button>
                    )}
                  </footer>
                </>
              )
            )}
          </div>
        </Popover>
      )}
    </>
  )
}

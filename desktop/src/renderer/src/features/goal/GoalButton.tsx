import { useRef, useState } from 'react'
import { Target, Check } from 'lucide-react'
import { Popover } from '../../components/Popover'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'
import { useChatGoal } from './useChatGoal'
import { GoalEditor } from './GoalEditor'
import { GoalStatus } from './GoalStatus'
import './goal.css'

export function GoalButton({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const anchor = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(false)
  const chatGoal = useChatGoal(w)
  const { goal, progress, phase, saving, busy, hasSession } = chatGoal
  const showEditor = editing || !goal
  const close = () => {
    setOpen(false)
    setEditing(false)
  }
  const label = !goal
    ? t('Цель')
    : phase === 'achieved'
      ? t('Цель достигнута')
      : progress.total
        ? `${t('Цель')} · ${progress.completed}/${progress.total}`
        : t('Цель')
  return (
    <>
      <button
        ref={anchor}
        className={`goal-button ${goal ? `has-goal ${phase}` : ''} ${open ? 'active-control' : ''}`}
        title={goal ? goal.text : t('Задать цель чата')}
        aria-label={t('Цель чата')}
        aria-expanded={open}
        onClick={() => (open ? close() : setOpen(true))}
      >
        {phase === 'achieved' ? <Check size={15} strokeWidth={2.5} /> : <Target size={15} />}
        <span>{label}</span>
        {goal && phase !== 'achieved' && progress.total > 0 && (
          <i className="goal-button-bar" aria-hidden="true">
            <b style={{ width: `${Math.round(progress.ratio * 100)}%` }} />
          </i>
        )}
      </button>
      {open && (
        <Popover anchor={anchor} close={close} width={360} label={t('Цель чата')} align="end">
          <div className="goal-panel">
            <header className="goal-panel-head">
              <Target size={15} />
              <strong>{t('Цель чата')}</strong>
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
                  <GoalStatus goal={goal} phase={phase} progress={progress} />
                  <div className="goal-actions">
                    <button className="quiet danger" onClick={() => void chatGoal.clear()}>
                      {t('Снять цель')}
                    </button>
                    <button className="quiet" onClick={() => setEditing(true)}>
                      {t('Изменить')}
                    </button>
                    {phase === 'paused' && progress.completed < progress.total && (
                      <button
                        className="primary"
                        onClick={() => {
                          close()
                          void chatGoal.resume(t('Продолжи'))
                        }}
                      >
                        {t('Продолжить')}
                      </button>
                    )}
                  </div>
                </>
              )
            )}
          </div>
        </Popover>
      )}
    </>
  )
}

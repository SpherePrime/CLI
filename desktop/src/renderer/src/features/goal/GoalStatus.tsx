import { Check } from 'lucide-react'
import { useI18n } from '../../i18n'
import type { ChatGoal, GoalPhase, GoalProgress } from '../../../../shared/chatGoal'

const phaseLabels: Record<GoalPhase, string> = {
  none: '',
  planning: 'Уточнение и план',
  working: 'В работе',
  verifying: 'Финальная проверка',
  paused: 'Остановлено',
  achieved: 'Цель достигнута'
}

export function GoalStatus({
  goal,
  phase,
  progress
}: {
  goal: ChatGoal
  phase: GoalPhase
  progress: GoalProgress
}) {
  const { t } = useI18n()
  return (
    <div className="goal-status">
      <div className={`goal-phase ${phase}`}>
        {phase === 'achieved' && <Check size={13} strokeWidth={3} />}
        {t(phaseLabels[phase])}
        {progress.total > 0 && (
          <span className="goal-count">
            {progress.completed}/{progress.total}
          </span>
        )}
      </div>
      <p className="goal-text">{goal.text}</p>
      {progress.total > 0 && (
        <div
          className="goal-progress"
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={progress.total}
          aria-valuenow={progress.completed}
        >
          <span style={{ width: `${Math.round(progress.ratio * 100)}%` }} />
        </div>
      )}
      {phase !== 'achieved' && progress.current && (
        <p className="goal-current">
          {t('Этап {current} из {total}: {title}', {
            current: Math.min(progress.completed + 1, progress.total),
            total: progress.total,
            title: progress.current
          })}
        </p>
      )}
      {phase === 'achieved' && goal.summary && <p className="goal-summary">{goal.summary}</p>}
    </div>
  )
}

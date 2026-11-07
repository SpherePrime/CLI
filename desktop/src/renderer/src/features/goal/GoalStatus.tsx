import { Check } from 'lucide-react'
import { useI18n } from '../../i18n'
import type { ChatGoal, GoalPhase, GoalProgress } from '../../../../shared/chatGoal'

type GoalStage = { content?: string; status?: string; active_form?: string }

const phaseLabels: Record<GoalPhase, string> = {
  none: '',
  planning: 'Уточнение и план',
  working: 'В работе',
  verifying: 'Финальная приёмка',
  paused: 'Остановлено',
  achieved: 'Цель достигнута'
}

export function GoalStatus({
  goal,
  phase,
  progress,
  stages
}: {
  goal: ChatGoal
  phase: GoalPhase
  progress: GoalProgress
  stages: GoalStage[]
}) {
  const { t } = useI18n()
  return (
    <div className="goal-status">
      <div className="goal-status-row">
        <span className={`goal-phase ${phase}`}>
          <i aria-hidden="true" />
          {t(phaseLabels[phase])}
        </span>
        {progress.total > 0 && (
          <span className="goal-count">
            {t('{done} из {total} этапов', { done: progress.completed, total: progress.total })}
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
      {stages.length > 0 && (
        <ol className="goal-stages">
          {stages.map((stage, index) => (
            <li key={index} className={stage.status}>
              <span className="goal-stage-marker">
                {stage.status === 'completed' ? <Check size={10} strokeWidth={3} /> : index + 1}
              </span>
              <span className="goal-stage-text">
                {stage.status === 'in_progress'
                  ? stage.active_form || stage.content
                  : stage.content}
              </span>
            </li>
          ))}
        </ol>
      )}
      {phase === 'achieved' && goal.summary && <p className="goal-summary">{goal.summary}</p>}
    </div>
  )
}

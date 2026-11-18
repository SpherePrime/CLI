import { useRef, useState, type RefObject } from 'react'
import { ChevronRight, Compass, Cpu, RotateCcw, Zap } from 'lucide-react'
import { Popover } from '../../components/Popover'
import { useI18n } from '../../i18n'
import { reasoningLabel, reasoningLevels } from './reasoning'

export type ReasoningOrchestratorView = {
  enabled: boolean
  plannerName: string
  plannerReasoning: string
  executorName: string
  executorReasoning: string
  onOpenOrchestrator: () => void
  onDisable: () => void
}

export function ReasoningPopover({
  anchor,
  close,
  modelName,
  mode,
  disabled,
  onChange,
  onReset,
  onOpenModels,
  orchestrator
}: {
  anchor: RefObject<HTMLElement | null>
  close: () => void
  modelName: string
  mode: string
  disabled: boolean
  onChange: (mode: string) => void
  onReset: () => void
  onOpenModels: () => void
  orchestrator?: ReasoningOrchestratorView
}) {
  const { t } = useI18n()
  const [preview, setPreview] = useState<string | null>(null)
  const shown = preview || mode
  const index = Math.max(0, reasoningLevels.indexOf(shown as (typeof reasoningLevels)[number]))
  const last = reasoningLevels.length - 1
  const track = useRef<HTMLDivElement>(null)
  const pick = (clientX: number) => {
    const rect = track.current?.getBoundingClientRect()
    if (!rect) return shown
    const ratio = Math.min(1, Math.max(0, (clientX - rect.left) / rect.width))
    return reasoningLevels[Math.round(ratio * last)]
  }
  const commit = (value: string) => {
    setPreview(null)
    if (value !== mode) onChange(value)
  }

  if (orchestrator?.enabled) {
    return (
      <Popover anchor={anchor} close={close} width={310} align="end" label={t('Режим Оркестратора')}>
        <div className="reasoning-popover orchestrator-popover">
          <div className="orchestrator-popover-head">
            <strong>{t('Оркестратор')}</strong>
            <span className="orchestrator-popover-badge">{t('План → Код')}</span>
          </div>
          <div className="orchestrator-popover-roles">
            <button
              type="button"
              className="orchestrator-popover-row"
              onClick={orchestrator.onOpenOrchestrator}
            >
              <span className="orchestrator-popover-role-label">
                <Compass size={13} aria-hidden="true" />
                <span>{t('Планировщик')}</span>
              </span>
              <span className="orchestrator-popover-role-value">
                <strong>{orchestrator.plannerName || t('Не выбрана')}</strong>
                <span>· {reasoningLabel(orchestrator.plannerReasoning, t)}</span>
              </span>
            </button>
            <button
              type="button"
              className="orchestrator-popover-row"
              onClick={orchestrator.onOpenOrchestrator}
            >
              <span className="orchestrator-popover-role-label">
                <Cpu size={13} aria-hidden="true" />
                <span>{t('Исполнитель')}</span>
              </span>
              <span className="orchestrator-popover-role-value">
                <strong>{orchestrator.executorName || t('Не выбрана')}</strong>
                <span>· {reasoningLabel(orchestrator.executorReasoning, t)}</span>
              </span>
            </button>
          </div>
          <div className="orchestrator-popover-actions">
            <button
              type="button"
              className="orchestrator-popover-disable"
              disabled={disabled}
              onClick={orchestrator.onDisable}
            >
              {t('Отключить')}
            </button>
            <button
              type="button"
              className="orchestrator-popover-configure"
              onClick={orchestrator.onOpenOrchestrator}
            >
              <span>{t('Настроить')}</span>
              <ChevronRight size={14} aria-hidden="true" />
            </button>
          </div>
        </div>
      </Popover>
    )
  }

  return (
    <Popover anchor={anchor} close={close} width={300} align="end" label={t('Уровень размышлений')}>
      <div className="reasoning-popover">
        <div className="reasoning-popover-head">
          <button
            className="icon small"
            title={t('Быстрее')}
            aria-label={t('Быстрее')}
            disabled={disabled}
            onClick={() => commit('low')}
          >
            <Zap size={15} />
          </button>
          <div className="reasoning-popover-title">
            <strong>{reasoningLabel(shown, t)}</strong>
            <button className="reasoning-popover-model" onClick={onOpenModels}>
              <span>{modelName}</span>
              <ChevronRight size={13} />
            </button>
          </div>
          <button
            className="icon small"
            title={t('По умолчанию')}
            aria-label={t('По умолчанию')}
            disabled={disabled}
            onClick={onReset}
          >
            <RotateCcw size={15} />
          </button>
        </div>
        <div
          ref={track}
          className={`reasoning-slider ${disabled ? 'disabled' : ''}`}
          role="slider"
          tabIndex={disabled ? -1 : 0}
          aria-label={t('Уровень размышлений')}
          aria-valuemin={0}
          aria-valuemax={last}
          aria-valuenow={index}
          aria-valuetext={reasoningLabel(shown, t)}
          aria-disabled={disabled}
          onPointerDown={(event) => {
            if (disabled) return
            event.currentTarget.setPointerCapture(event.pointerId)
            setPreview(pick(event.clientX))
          }}
          onPointerMove={(event) => {
            if (disabled || !event.currentTarget.hasPointerCapture(event.pointerId)) return
            setPreview(pick(event.clientX))
          }}
          onPointerUp={(event) => {
            if (disabled || !event.currentTarget.hasPointerCapture(event.pointerId)) return
            event.currentTarget.releasePointerCapture(event.pointerId)
            commit(pick(event.clientX))
          }}
          onPointerCancel={() => setPreview(null)}
          onLostPointerCapture={() => setPreview(null)}
          onKeyDown={(event) => {
            if (disabled) return
            const step = { ArrowLeft: -1, ArrowDown: -1, ArrowRight: 1, ArrowUp: 1 }[event.key]
            if (event.key === 'Home') commit(reasoningLevels[0])
            else if (event.key === 'End') commit(reasoningLevels[last])
            else if (step) commit(reasoningLevels[Math.min(last, Math.max(0, index + step))])
            else return
            event.preventDefault()
          }}
        >
          <div
            className="reasoning-slider-fill"
            style={{ width: `calc(28px + (100% - 28px) * ${index / last})` }}
          />
          {reasoningLevels.map((level, position) => (
            <span
              key={level}
              className={`reasoning-slider-dot ${position <= index ? 'active' : ''}`}
              style={{ left: `calc(14px + (100% - 28px) * ${position / last})` }}
            />
          ))}
          <span
            className="reasoning-slider-thumb"
            style={{ left: `calc(14px + (100% - 28px) * ${index / last})` }}
          />
        </div>
      </div>
    </Popover>
  )
}

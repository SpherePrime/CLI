import { useRef, useState, type RefObject } from 'react'
import { ChevronRight, RotateCcw, Zap } from 'lucide-react'
import { Popover } from '../../components/Popover'
import { useI18n } from '../../i18n'
import { reasoningLabel, reasoningLevels } from './reasoning'

export function ReasoningPopover({
  anchor,
  close,
  modelName,
  mode,
  disabled,
  onChange,
  onReset,
  onOpenModels
}: {
  anchor: RefObject<HTMLElement | null>
  close: () => void
  modelName: string
  mode: string
  disabled: boolean
  onChange: (mode: string) => void
  onReset: () => void
  onOpenModels: () => void
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
            style={{ width: `calc((100% - 28px) * ${index / last})` }}
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

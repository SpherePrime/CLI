import type { ComponentType, ReactNode } from 'react'
import '../styles/empty.css'

export function EmptyState({
  icon: Icon,
  title,
  text,
  action,
  note,
  compact = false
}: {
  icon: ComponentType<{ size?: number }>
  title: string
  text?: string
  action?: { label: string; icon?: ComponentType<{ size?: number }>; onClick(): void; disabled?: boolean }
  note?: ReactNode
  compact?: boolean
}) {
  const ActionIcon = action?.icon
  return (
    <div className={`empty-state ${compact ? 'compact' : ''}`}>
      <span className="empty-state-icon">
        <Icon size={compact ? 20 : 24} />
      </span>
      <h3>{title}</h3>
      {text && <p>{text}</p>}
      {action && (
        <button className="empty-state-action" onClick={action.onClick} disabled={action.disabled}>
          {ActionIcon && <ActionIcon size={14} />}
          {action.label}
        </button>
      )}
      {note && <div className="empty-state-note">{note}</div>}
    </div>
  )
}

import { useEffect, useState } from 'react'
import type { WindowShare, WindowState } from '../../../../shared/computer'
import { useI18n } from '../../i18n'
import '../../styles/window-control-badge.css'

export function WindowControlNotice({
  title,
  revoke,
  label,
  revokeLabel
}: {
  title: string
  revoke(): void
  label: string
  revokeLabel: string
}) {
  return (
    <div className="window-control-notice" role="status">
      <span className="window-control-dot" />
      <span className="window-control-title" title={title}>
        {label}: {title}
      </span>
      <button type="button" onClick={revoke}>
        {revokeLabel}
      </button>
      <kbd>Ctrl+Alt+Escape</kbd>
    </div>
  )
}

export function WindowControlBadge({
  workspaceIds,
  onError
}: {
  workspaceIds: string[]
  onError(error: unknown): void
}) {
  const { t } = useI18n()
  const [shares, setShares] = useState<Record<string, WindowShare>>({})
  const key = workspaceIds.join(',')
  useEffect(() => {
    let disposed = false
    const observed = new Set<string>()
    const update = (workspaceId: string, state: WindowState) =>
      setShares((current) => {
        const next = { ...current }
        if (state.share?.mode === 'control') next[workspaceId] = state.share
        else delete next[workspaceId]
        return next
      })
    const unsubscribe = window.prime.onEvent((event) => {
      if (event.type === 'windows' && event.workspaceId) {
        observed.add(event.workspaceId)
        update(event.workspaceId, event.payload)
      }
    })
    for (const workspaceId of workspaceIds)
      void window.prime
        .windows(workspaceId, { action: 'state' })
        .then((state) => {
          if (!disposed && !observed.has(workspaceId)) update(workspaceId, state)
        })
        .catch(() => {})
    return () => {
      disposed = true
      unsubscribe()
    }
  }, [key])
  const active = Object.values(shares).filter((share) => workspaceIds.includes(share.workspaceId))
  if (!active.length) return null
  return (
    <div className="window-control-badges">
      {active.map((share) => (
        <WindowControlNotice
          key={share.workspaceId}
          title={share.title}
          label={t('Управление окном')}
          revokeLabel={t('Отозвать')}
          revoke={() => {
            void window.prime.windows(share.workspaceId, { action: 'revoke' }).catch(onError)
          }}
        />
      ))}
    </div>
  )
}

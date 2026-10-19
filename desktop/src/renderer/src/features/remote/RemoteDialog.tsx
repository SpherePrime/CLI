import { useCallback } from 'react'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import type { DesktopEvent } from '../../../../shared/contracts'
import type { RemoteRequest } from '../../../../shared/remote'
import { RemotePanel } from './RemotePanel'

export function RemoteDialog({ workspace, close }: { workspace: Workspace; close(): void }) {
  const { t } = useI18n()
  const api = useCallback((request: RemoteRequest) => window.prime.remote(request), [])
  const subscribe = useCallback(
    (listener: (event: DesktopEvent) => void) => window.prime.onEvent(listener),
    []
  )
  const chooseKey = useCallback(() => window.prime.remoteKey(), [])
  return (
    <Modal title={t('SSH-серверы')} close={close} className="remote-dialog">
      <RemotePanel
        api={api}
        subscribe={subscribe}
        onError={() => {}}
        chooseKey={chooseKey}
        onOpenProject={(profileId) => {
          close()
          void workspace.openRemote(profileId)
        }}
      />
    </Modal>
  )
}

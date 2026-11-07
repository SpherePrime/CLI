import { useCallback, useEffect, useRef, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import type { AppUiIssue, PickedAppElement } from '../../../../shared/appTesting'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'
import { PickedAttachments } from '../browser/pickedAttachments'
import { ActivityLog } from './ActivityLog'
import { AppStatus } from './AppStatus'
import { ConnectWindow } from './ConnectWindow'
import { appTestingCopy } from './copy'
import { PickedElementCard } from './PickedElementCard'
import { SnapshotTree } from './SnapshotTree'
import { pickedFromNode } from './snapshotRows'
import { TestReport } from './TestReport'
import { useAppTesting } from './useAppTesting'
import '../../styles/app-testing.css'

type Props = {
  workspace: Workspace
  workspaceId: string
  sessionId?: string
  onSend(prompt: string, attachmentIds: string[]): Promise<unknown>
  onError(error: unknown): void
}

export function AppTestingPanel({ workspace, workspaceId, sessionId, onSend, onError }: Props) {
  const { language } = useI18n()
  const copy = appTestingCopy[language]
  const app = useAppTesting(workspaceId, sessionId, onError)
  const [picking, setPicking] = useState(false)
  const [picked, setPicked] = useState<PickedAppElement | null>(null)
  const [highlighted, setHighlighted] = useState<string[]>([])
  const attachments = useRef<PickedAttachments | null>(null)
  const active = useRef(workspaceId)
  active.current = workspaceId

  useEffect(() => {
    const owner = new PickedAttachments((ids) => {
      void window.prime.releaseAttachments(ids).catch(() => {})
    })
    attachments.current = owner
    return () => {
      owner.dispose()
      attachments.current = null
      void window.prime.appPickCancel().catch(() => {})
    }
  }, [])

  useEffect(() => {
    setPicked(null)
    setHighlighted([])
  }, [workspaceId])

  const pick = useCallback(async () => {
    if (!sessionId) {
      onError(new Error(copy.noChat))
      return
    }
    if (picking) {
      await window.prime.appPickCancel()
      return
    }
    setPicking(true)
    try {
      const result = await window.prime.appPick(workspaceId, sessionId)
      if (active.current !== workspaceId || !result) return
      attachments.current?.replace(result.attachmentId ? [result.attachmentId] : [])
      setPicked(result)
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause)
      if (!message.includes('cancelled')) onError(cause)
    } finally {
      setPicking(false)
    }
  }, [copy.noChat, onError, picking, sessionId, workspaceId])

  const showIssue = (issue: AppUiIssue) => {
    setHighlighted(issue.refs)
    app.setTreeOpen(true)
  }

  const attached = !!app.state.target
  return (
    <div className="app-testing-panel">
      <AppStatus
        copy={copy}
        state={app.state}
        picking={picking}
        busy={app.busy}
        onPick={() => void pick()}
        onDetach={() => void app.detach()}
      />
      {picking && <p className="app-testing-hint">{copy.pickHint}</p>}
      {picked && (
        <PickedElementCard
          copy={copy}
          workspace={workspace}
          picked={picked}
          onSend={async (prompt, ids) => {
            const owner = attachments.current
            owner?.beginSend(ids)
            try {
              return await onSend(prompt, ids)
            } finally {
              owner?.endSend(ids)
            }
          }}
          onClose={() => {
            attachments.current?.replace([])
            setPicked(null)
          }}
        />
      )}
      <TestReport copy={copy} report={app.state.report} onClear={() => void app.clearReport()} onIssue={showIssue} />
      {attached && (
        <section className="app-testing-section">
          <button
            type="button"
            className="app-testing-disclosure"
            onClick={() => app.setTreeOpen(!app.treeOpen)}
          >
            {app.treeOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
            {copy.elements}
            {app.snapshot && <span className="app-testing-muted">{app.snapshot.count}</span>}
          </button>
          {app.treeOpen && (
            <SnapshotTree
              copy={copy}
              snapshot={app.snapshot}
              highlighted={highlighted}
              onPick={(node) => {
                if (!app.snapshot) return
                attachments.current?.replace([])
                setPicked(pickedFromNode(node, app.snapshot))
              }}
            />
          )}
        </section>
      )}
      <ActivityLog copy={copy} log={app.state.log ?? []} />
      {!attached && (
        <ConnectWindow
          copy={copy}
          windows={app.windows}
          busy={app.busy}
          onOpen={() => void app.refreshWindows()}
          onRefresh={() => void app.refreshWindows()}
          onAttach={(target) => void app.attach(target)}
        />
      )}
    </div>
  )
}

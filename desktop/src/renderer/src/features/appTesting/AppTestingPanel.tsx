import { useCallback, useEffect, useRef, useState } from 'react'
import { Crosshair } from 'lucide-react'
import type {
  AppUiElementAction,
  AppUiExplore,
  AppUiLogEntry,
  AppUiRecord,
  AppUiRequest,
  AppUiRun,
  AppUiScenario,
  AppUiSnapshot,
  AppUiTarget,
  PickedAppElement
} from '../../../../shared/appTesting'
import { useI18n } from '../../i18n'
import { PickedAttachments } from '../browser/pickedAttachments'
import { appTestingCopy } from './copy'
import { PickedElementCard } from './PickedElementCard'
import { SnapshotTree } from './SnapshotTree'
import { TargetList } from './TargetList'
import '../../styles/app-testing.css'

type Props = {
  workspaceId: string
  sessionId?: string
  onSend(prompt: string, attachmentIds: string[]): Promise<unknown>
  onError(error: unknown): void
}

export function AppTestingPanel({ workspaceId, sessionId, onSend, onError }: Props) {
  const { language } = useI18n()
  const c = appTestingCopy[language]
  const [targets, setTargets] = useState<AppUiTarget[]>([])
  const [target, setTarget] = useState<AppUiTarget | null>(null)
  const [snapshot, setSnapshot] = useState<AppUiSnapshot | null>(null)
  const [log, setLog] = useState<AppUiLogEntry[]>([])
  const [scenarios, setScenarios] = useState<AppUiScenario[]>([])
  const [result, setResult] = useState<AppUiRun | null>(null)
  const [explored, setExplored] = useState<AppUiExplore | null>(null)
  const [recorded, setRecorded] = useState<AppUiRecord | null>(null)
  const [recording, setRecording] = useState(false)
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState('')
  const [text, setText] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [picking, setPicking] = useState(false)
  const [picked, setPicked] = useState<PickedAppElement | null>(null)
  const attachments = useRef<PickedAttachments | null>(null)
  const active = useRef(workspaceId)
  active.current = workspaceId

  const call = useCallback(
    (request: AppUiRequest) => window.prime.appTesting(workspaceId, request),
    [workspaceId]
  )

  const fail = useCallback(
    (cause: unknown) => {
      setError(cause instanceof Error ? cause.message : String(cause))
      onError(cause)
    },
    [onError]
  )

  const refresh = useCallback(async () => {
    setBusy(true)
    setError('')
    try {
      const [list, state, tests] = await Promise.all([
        call({ action: 'targets' }),
        call({ action: 'state' }),
        call({ action: 'testList' })
      ])
      if (active.current !== workspaceId) return
      setTargets(list.windows ?? [])
      setTarget(state.target ?? null)
      setLog(state.log ?? [])
      setScenarios(tests.scenarios ?? [])
    } catch (cause) {
      fail(cause)
    } finally {
      setBusy(false)
    }
  }, [call, fail, workspaceId])

  useEffect(() => {
    setTargets([])
    setTarget(null)
    setSnapshot(null)
    setSelected('')
    setLog([])
    setScenarios([])
    setResult(null)
    setExplored(null)
    setRecorded(null)
    setRecording(false)
    void refresh()
    return window.prime.onEvent((event) => {
      if (event.type === 'appui' && event.workspaceId === workspaceId) {
        setTarget(event.payload?.target ?? null)
        setLog(event.payload?.log ?? [])
      }
    })
  }, [refresh, workspaceId])

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
    if (!picking) return
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') void window.prime.appPickCancel()
    }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  }, [picking])

  const pick = async () => {
    if (!sessionId) {
      setError(c.noChat)
      return
    }
    if (picking) {
      await window.prime.appPickCancel()
      return
    }
    setPicking(true)
    setError('')
    try {
      const result = await window.prime.appPick(workspaceId, sessionId)
      if (active.current !== workspaceId || !result) return
      attachments.current?.replace(result.attachmentId ? [result.attachmentId] : [])
      setPicked(result)
    } catch (cause) {
      fail(cause)
    } finally {
      setPicking(false)
    }
  }

  const run = useCallback(
    async (request: AppUiRequest) => {
      if (!sessionId) {
        setError(c.noChat)
        return
      }
      setBusy(true)
      setError('')
      try {
        const result = await call(request)
        if (active.current !== workspaceId) return
        if (result.snapshot) setSnapshot(result.snapshot)
        if (result.windows) setTargets(result.windows)
        if (result.target !== undefined) setTarget(result.target)
        if (result.log) setLog(result.log)
        if (result.scenarios) setScenarios(result.scenarios)
        if (result.run) setResult(result.run)
        if (result.explore) setExplored(result.explore)
        if (result.record) setRecorded(result.record)
      } catch (cause) {
        fail(cause)
      } finally {
        setBusy(false)
      }
    },
    [call, c.noChat, fail, sessionId, workspaceId]
  )

  const attach = (item: AppUiTarget, mode: 'view' | 'control') =>
    void run({
      action: 'attach',
      sessionId: sessionId || '',
      hwnd: item.hwnd,
      pid: item.pid,
      identity: item.identity || '',
      title: item.title,
      mode
    })

  const act = (elementAction: AppUiElementAction) =>
    void run({ action: 'act', sessionId: sessionId || '', locator: { reference: selected }, elementAction })

  return (
    <div className="automation-panel app-testing-panel">
      <div className="automation-header">
        <strong>{c.title}</strong>
      </div>
      {error && (
        <div className="windows-error" role="alert">
          {error}
        </div>
      )}
      <div className="app-testing-layout">
        <TargetList
          language={language}
          targets={targets}
          attached={target}
          busy={busy}
          onRefresh={() => void refresh()}
          onAttach={attach}
          onDetach={() => void run({ action: 'detach' })}
        />
        <div className="app-testing-detail">
          <div className="app-testing-toolbar">
            <input
              className="app-testing-search"
              value={query}
              placeholder={c.search}
              aria-label={c.search}
              onChange={(event) => setQuery(event.target.value)}
            />
            <button
              type="button"
              disabled={busy || !target}
              onClick={() => void run({ action: 'snapshot', sessionId: sessionId || '' })}
            >
              {c.snapshot}
            </button>
            <button
              className={`icon small ${picking ? 'active' : ''}`}
              type="button"
              aria-label={c.pick}
              aria-pressed={picking}
              title={c.pick}
              disabled={busy || !target}
              onClick={() => void pick()}
            >
              <Crosshair size={14} />
            </button>
            <button
              type="button"
              disabled={busy || !target || !sessionId}
              onClick={() => void run({ action: 'explore', sessionId: sessionId || '' })}
            >
              {c.explore}
            </button>
            <button
              className={recording ? 'active' : ''}
              type="button"
              disabled={!target || !sessionId}
              onClick={async () => {
                if (recording) {
                  setRecording(false)
                  await run({ action: 'recordStop', sessionId: sessionId || '' })
                } else {
                  setRecording(true)
                  await run({ action: 'recordStart' })
                }
              }}
            >
              {recording ? c.recordStop : c.record}
            </button>
          </div>
          {recording && (
            <p className="app-testing-warning" role="status">
              {c.recording}
            </p>
          )}
          {picking && (
            <p className="app-testing-warning" role="status">
              {c.pickHint}
            </p>
          )}
          {picked && (
            <PickedElementCard
              language={language}
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
          <SnapshotTree
            language={language}
            snapshot={snapshot}
            query={query}
            selected={selected}
            onSelect={setSelected}
          />
          {selected && (
            <div className="app-testing-actions">
              <span className="app-testing-actions-title">{c.actions}</span>
              <button type="button" disabled={busy} onClick={() => act('click')}>
                {c.click}
              </button>
              <button type="button" disabled={busy} onClick={() => act('toggle')}>
                {c.toggle}
              </button>
              <button type="button" disabled={busy} onClick={() => act('select')}>
                {c.select}
              </button>
              <button type="button" disabled={busy} onClick={() => act('focus')}>
                {c.focus}
              </button>
              <input
                value={text}
                aria-label={c.type}
                onChange={(event) => setText(event.target.value)}
              />
              <button
                type="button"
                disabled={busy || !text}
                onClick={() =>
                  void run({
                    action: 'type',
                    sessionId: sessionId || '',
                    locator: { reference: selected },
                    text
                  })
                }
              >
                {c.type}
              </button>
            </div>
          )}
          <div className="app-testing-log">
            <span className="app-testing-actions-title">{c.log}</span>            {log.length === 0 ? (
              <p className="app-testing-empty">{c.noLog}</p>
            ) : (
              <ul>
                {log
                  .slice()
                  .reverse()
                  .slice(0, 20)
                  .map((entry, index) => (
                    <li key={`${entry.time}-${index}`} className={entry.ok ? '' : 'failed'}>
                      <span>{entry.action}</span>
                      <span className="app-testing-log-title">{entry.title}</span>
                      {entry.error && <span className="app-testing-log-error">{entry.error}</span>}
                    </li>
                  ))}
              </ul>
            )}
          </div>
          <div className="app-testing-scenarios">
            <div className="app-testing-toolbar">
              <span className="app-testing-actions-title">{c.scenarios}</span>
              <button type="button" disabled={busy} onClick={() => void run({ action: 'testList' })}>
                {c.refresh}
              </button>
            </div>
            {scenarios.length === 0 ? (
              <p className="app-testing-empty">{c.noScenarios}</p>
            ) : (
              <ul>
                {scenarios.map((scenario) => (
                  <li key={scenario.path} className={scenario.error ? 'failed' : ''}>
                    <span className="app-testing-log-title">{scenario.name || scenario.path}</span>
                    <span className="app-testing-target-meta">{scenario.steps ?? 0}</span>
                    <button
                      type="button"
                      disabled={busy || !!scenario.error || !sessionId}
                      onClick={() =>
                        void run({
                          action: 'testRun',
                          sessionId: sessionId || '',
                          path: scenario.path
                        })
                      }
                    >
                      {c.run}
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {recorded && (
              <div className="app-testing-run">
                <strong>
                  {c.draft}: {recorded.path} · {recorded.steps}
                </strong>
              </div>
            )}
            {explored && (
              <div className="app-testing-run">
                <strong>
                  {c.explore}: {explored.visited} · {c.stopped}: {explored.stopped}
                </strong>
                <ul>
                  {explored.steps.map((step) => (
                    <li key={step.index} className={step.ok ? '' : 'failed'}>
                      <span>
                        {step.index}. {step.role} "{step.name}"
                      </span>
                      {step.error && <span className="app-testing-log-error">{step.error}</span>}
                    </li>
                  ))}
                </ul>
                {explored.blocked.length > 0 && (
                  <span className="app-testing-log-title">
                    {c.blocked}:{' '}
                    {explored.blocked.map((item) => `${item.role} "${item.name}"`).join(', ')}
                  </span>
                )}
              </div>
            )}
            {result && (
              <div className="app-testing-run">
                <strong className={result.passed ? '' : 'failed'}>
                  {result.passed ? c.passed : c.failed}
                </strong>
                <span className="app-testing-log-title" title={result.artifacts}>
                  {c.artifacts}: {result.artifacts}
                </span>
                <ul>
                  {result.steps.map((step) => (
                    <li key={step.index} className={step.ok ? '' : 'failed'}>
                      <span>
                        {step.index}. {step.action}
                      </span>
                      <span className="app-testing-target-meta">{step.ms} ms</span>
                      {step.error && <span className="app-testing-log-error">{step.error}</span>}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

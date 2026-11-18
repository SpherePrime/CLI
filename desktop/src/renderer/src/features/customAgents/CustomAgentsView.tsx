import { useMemo, useState } from 'react'
import {
  Activity,
  Bot,
  Brain,
  GitBranch,
  History,
  Layers,
  PanelLeft,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  SlidersHorizontal,
  Trash2,
  Users
} from 'lucide-react'
import type { CustomWorkflow, SwarmResult } from '../../../../shared/customAgents'
import type { Workspace } from '../../app/useWorkspace'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import { AgentInspector } from './AgentInspector'
import { AgentRoster } from './AgentRoster'
import {
  AgentWorkspace,
  type RunsSubTab,
  type WorkspaceTab
} from './AgentWorkspace'
import { CreateAgentDialog } from './CreateAgentDialog'
import { CreateWorkflowDialog } from './CreateWorkflowDialog'
import { GlobalMemoryDialog } from './GlobalMemoryDialog'
import type { AgentPreset, RoleFilter } from './presentation'
import { SwarmDialog } from './SwarmDialog'
import { useCustomAgents } from './useCustomAgents'
import type { PendingWorkflowLink } from './WorkflowCanvas'
import './customAgents.css'

type MobilePane = 'roster' | 'workspace' | 'inspector'

export function CustomAgentsView({ workspace }: { workspace: Workspace }) {
  const { t } = useI18n()
  const state = useCustomAgents()
  const [query, setQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState<RoleFilter>('all')
  const [createOpen, setCreateOpen] = useState(false)
  const [pendingLink, setPendingLink] = useState<PendingWorkflowLink | null>(null)
  const [workflowDialogMode, setWorkflowDialogMode] = useState<'closed' | 'create' | 'edit'>('closed')
  const [confirmDeleteWorkflow, setConfirmDeleteWorkflow] = useState(false)
  const [deleteWorkflowAgents, setDeleteWorkflowAgents] = useState(false)
  const [runningWorkflow, setRunningWorkflow] = useState(false)
  const [memoryOpen, setMemoryOpen] = useState(false)
  const [swarmOpen, setSwarmOpen] = useState(false)
  const [lastSwarmResult, setLastSwarmResult] = useState<SwarmResult | null>(null)
  const [rosterOpen, setRosterOpen] = useState(true)
  const [inspectorOpen, setInspectorOpen] = useState(false)
  const [workspaceTab, setWorkspaceTab] = useState<WorkspaceTab>('chain')
  const [requestedRunsSubTab, setRequestedRunsSubTab] = useState<RunsSubTab | undefined>(undefined)
  const [mobilePane, setMobilePane] = useState<MobilePane>('workspace')

  const selected = state.selectedAgent
  const latestRun = state.agentRuns[0] || null
  const activeWorkflow = state.activeWorkflow
  const activeWorkflowId = activeWorkflow?.id || state.activeWorkflowId || 'default'

  const workflowNodeCounts = useMemo(() => {
    const counts = new Map<string, number>()
    for (const agent of state.agents) {
      const wfId = agent.workflow_id || 'default'
      counts.set(wfId, (counts.get(wfId) || 0) + 1)
    }
    return counts
  }, [state.agents])

  const activeWorkflowAgentsCount = workflowNodeCounts.get(activeWorkflowId) || 0

  const openCreateDialog = (link?: PendingWorkflowLink) => {
    setPendingLink(link || null)
    setCreateOpen(true)
  }

  const openSettingsForAgent = (agentId: string) => {
    state.setSelectedAgentId(agentId)
    setInspectorOpen(true)
    setMobilePane('inspector')
  }

  const handleQuickCreateStarter = async (preset: AgentPreset) => {
    const created = await state.createAgent({
      name: preset.name,
      role: preset.role,
      description: preset.description,
      trigger_type: preset.trigger_type,
      interval_minutes: preset.interval_minutes,
      monitor_kind: preset.monitor_kind,
      monitor_target: preset.monitor_target,
      auto_model_hint: preset.auto_model_hint,
      system_prompt: preset.system_prompt,
      acceptance_criteria: preset.acceptance_criteria,
      deterministic_checks: preset.deterministic_checks,
      workspace_path: workspace.project?.kind === 'project' ? workspace.project.path : '',
      workflow_id: activeWorkflowId,
      enabled: true
    })
    if (activeWorkflow && !activeWorkflow.entry_agent_id) {
      await state.updateWorkflow(activeWorkflow.id, { entry_agent_id: created.id })
    }
  }

  const handleRunWorkflow = async () => {
    if (!activeWorkflow || runningWorkflow || activeWorkflowAgentsCount === 0) return
    setRunningWorkflow(true)
    try {
      await state.runWorkflow(activeWorkflow.id)
      setWorkspaceTab('tasks')
      setMobilePane('workspace')
    } finally {
      setRunningWorkflow(false)
    }
  }

  const canDeleteActiveWorkflow = Boolean(activeWorkflow && activeWorkflow.id !== 'default')

  return (
    <div className="ca-studio" data-mobile-pane={mobilePane} aria-label={t('Свои агенты')}>
      <header className="ca-studio-head">
        <div className="ca-studio-head-left">
          <h1>{t('Студия агентов')}</h1>
          <div className="ca-mode-tabs" role="tablist" aria-label={t('Режим студии')}>
            <button
              type="button"
              role="tab"
              aria-selected={workspaceTab === 'chain'}
              className={workspaceTab === 'chain' ? 'active' : ''}
              onClick={() => {
                setWorkspaceTab('chain')
                setMobilePane('workspace')
              }}
            >
              <GitBranch size={13} aria-hidden="true" />
              <span>{t('Цепочка')}</span>
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={workspaceTab === 'tasks'}
              className={workspaceTab === 'tasks' ? 'active' : ''}
              onClick={() => {
                setWorkspaceTab('tasks')
                setMobilePane('workspace')
              }}
            >
              <Activity size={13} aria-hidden="true" />
              <span>{t('Задачи и чат')}</span>
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={workspaceTab === 'runs'}
              className={workspaceTab === 'runs' ? 'active' : ''}
              onClick={() => {
                setWorkspaceTab('runs')
                setMobilePane('workspace')
              }}
            >
              <History size={13} aria-hidden="true" />
              <span>{t('Запуски и доска')}</span>
              <span className="ca-tab-count">{state.runs.length}</span>
            </button>
          </div>
        </div>

        <div className="ca-studio-head-actions">
          {state.globalMemory && (
            <span className="ca-studio-mem-pill">
              {`MEMORY ${state.globalMemory.memory_chars}/${state.globalMemory.memory_limit}`}
            </span>
          )}
          <button
            type="button"
            className="ca-head-btn"
            onClick={() => setMemoryOpen(true)}
          >
            <Brain size={14} aria-hidden="true" />
            <span>{t('Память и личность')}</span>
          </button>
          <button
            type="button"
            className="ca-head-btn"
            onClick={() => setSwarmOpen(true)}
          >
            <Layers size={14} aria-hidden="true" />
            <span>{t('Рой')}</span>
          </button>
          <button
            type="button"
            className={`ca-head-btn ca-roster-toggle ${rosterOpen ? 'active' : ''}`}
            aria-pressed={rosterOpen}
            onClick={() => setRosterOpen((prev) => !prev)}
          >
            <PanelLeft size={14} aria-hidden="true" />
            <span>{t('Ростер')}</span>
          </button>
          <button
            type="button"
            className={`ca-head-btn ca-inspector-toggle ${inspectorOpen ? 'active' : ''}`}
            aria-pressed={inspectorOpen}
            disabled={!selected}
            onClick={() => setInspectorOpen((prev) => !prev)}
          >
            <SlidersHorizontal size={14} aria-hidden="true" />
            <span>{t('Настройки')}</span>
          </button>
        </div>
      </header>

      <div className="ca-wf-bar" aria-label={t('Управление цепочками')}>
        <div className="ca-wf-bar-left">
          <span className="ca-wf-bar-label">{t('Цепочка:')}</span>
          <div className="ca-wf-switcher">
            <Select
              value={activeWorkflowId}
              onChange={(value) => {
                state.setActiveWorkflowId(value)
                setConfirmDeleteWorkflow(false)
              }}
              label={t('Активная цепочка')}
              options={
                state.workflows.length > 0
                  ? state.workflows.map((wf) => ({
                      value: wf.id,
                      label: `${wf.name} (${workflowNodeCounts.get(wf.id) || 0})`
                    }))
                  : [{ value: 'default', label: `${t('Основная цепочка')} (${activeWorkflowAgentsCount})` }]
              }
            />
          </div>
          <button
            type="button"
            className="ca-head-btn"
            onClick={() => {
              setConfirmDeleteWorkflow(false)
              setWorkflowDialogMode('create')
            }}
          >
            <Plus size={14} aria-hidden="true" />
            <span>{t('Новая цепочка')}</span>
          </button>
        </div>

        <div className="ca-wf-bar-actions">
          {activeWorkflow && (
            <>
              <button
                type="button"
                className="primary ca-head-btn"
                disabled={runningWorkflow || state.busy || activeWorkflowAgentsCount === 0}
                onClick={() => void handleRunWorkflow()}
              >
                <Play size={13} aria-hidden="true" />
                <span>{runningWorkflow ? t('Запуск…') : t('Запустить цепь')}</span>
              </button>
              <button
                type="button"
                className="ca-head-btn"
                title={t('Переименовать цепочку')}
                aria-label={t('Переименовать цепочку')}
                onClick={() => {
                  setConfirmDeleteWorkflow(false)
                  setWorkflowDialogMode('edit')
                }}
              >
                <Pencil size={13} aria-hidden="true" />
                <span>{t('Переименовать')}</span>
              </button>
              {canDeleteActiveWorkflow && !confirmDeleteWorkflow && (
                <button
                  type="button"
                  className="danger ca-head-btn"
                  title={t('Удалить цепочку')}
                  aria-label={t('Удалить цепочку')}
                  onClick={() => {
                    setDeleteWorkflowAgents(false)
                    setConfirmDeleteWorkflow(true)
                  }}
                >
                  <Trash2 size={13} aria-hidden="true" />
                  <span>{t('Удалить')}</span>
                </button>
              )}
            </>
          )}
        </div>
      </div>

      {confirmDeleteWorkflow && activeWorkflow && canDeleteActiveWorkflow && (
        <div className="ca-wf-delete-banner" role="alert">
          <div className="ca-wf-delete-copy">
            <strong>{t('Удалить цепочку «{name}»?', { name: activeWorkflow.name })}</strong>
            {activeWorkflowAgentsCount > 0 && (
              <label className="ca-checkbox">
                <input
                  type="checkbox"
                  checked={deleteWorkflowAgents}
                  onChange={(event) => setDeleteWorkflowAgents(event.target.checked)}
                />
                <span>
                  {t('Также удалить агентов ({count})', { count: activeWorkflowAgentsCount })}
                </span>
              </label>
            )}
          </div>
          <div className="ca-confirm-actions">
            <button type="button" onClick={() => setConfirmDeleteWorkflow(false)}>
              {t('Отмена')}
            </button>
            <button
              type="button"
              className="danger"
              disabled={state.busy}
              onClick={() => {
                void state
                  .deleteWorkflow(activeWorkflow.id, deleteWorkflowAgents)
                  .then(() => setConfirmDeleteWorkflow(false))
              }}
            >
              {t('Подтвердить')}
            </button>
          </div>
        </div>
      )}

      <div className="ca-mobile-tabs" role="tablist" aria-label={t('Панели студии')}>
        <button
          type="button"
          role="tab"
          aria-selected={mobilePane === 'roster'}
          className={mobilePane === 'roster' ? 'active' : ''}
          onClick={() => setMobilePane('roster')}
        >
          <Users size={14} aria-hidden="true" />
          <span>{t('Ростер')}</span>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={mobilePane === 'workspace'}
          className={mobilePane === 'workspace' ? 'active' : ''}
          onClick={() => setMobilePane('workspace')}
        >
          <Bot size={14} aria-hidden="true" />
          <span>{t('Студия')}</span>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={mobilePane === 'inspector'}
          className={mobilePane === 'inspector' ? 'active' : ''}
          disabled={!selected}
          onClick={() => {
            setInspectorOpen(true)
            setMobilePane('inspector')
          }}
        >
          <SlidersHorizontal size={14} aria-hidden="true" />
          <span>{t('Настройки')}</span>
        </button>
      </div>

      {state.error && (
        <div className="ca-top-error" role="alert">
          <span>{state.error}</span>
          <button type="button" className="ca-mini-btn" onClick={() => void state.refresh()}>
            <RefreshCw size={12} aria-hidden="true" />
            <span>{t('Повторить')}</span>
          </button>
        </div>
      )}

      <div
        className={`ca-grid ${rosterOpen ? '' : 'roster-collapsed'} ${inspectorOpen ? '' : 'inspector-collapsed'}`}
      >
        <div className="ca-pane ca-pane-roster">
          <AgentRoster
            agents={state.agents}
            workflows={state.workflows}
            activeWorkflowId={activeWorkflowId}
            models={state.models}
            selectedId={state.selectedAgentId}
            query={query}
            roleFilter={roleFilter}
            loading={state.loading}
            onSelect={(id) => {
              state.setSelectedAgentId(id)
              setMobilePane('workspace')
            }}
            onQueryChange={setQuery}
            onRoleFilterChange={setRoleFilter}
            onToggleEnabled={(agent) => {
              void state.updateAgent(agent.id, { enabled: !agent.enabled })
            }}
            onAttachToWorkflow={(agentId) => {
              void state.updateAgent(agentId, { workflow_id: activeWorkflowId })
            }}
            onOpenCreate={() => openCreateDialog()}
          />
        </div>

        <div className="ca-pane ca-pane-workspace">
          <AgentWorkspace
            agent={selected}
            agents={state.agents}
            activeWorkflow={activeWorkflow}
            models={state.models}
            runs={state.agentRuns}
            allRuns={state.runs}
            blackboard={state.blackboard}
            selectedChainId={state.selectedChainId}
            tab={workspaceTab}
            requestedRunsSubTab={requestedRunsSubTab}
            onTabChange={setWorkspaceTab}
            onSelectChainId={state.setSelectedChainId}
            onSelectAgent={state.setSelectedAgentId}
            onConfigureAgent={openSettingsForAgent}
            onRun={state.runAgent}
            onCancelRun={state.cancelRun}
            onProbe={state.probeAgent}
            onPostBlackboard={state.postBlackboard}
            onUpdateAgent={state.updateAgent}
            onSaveLayout={(workflowId, layoutJson) =>
              state.updateWorkflow(workflowId, { layout_json: layoutJson })
            }
            onSetEntryAgent={(workflowId, agentId) =>
              state.updateWorkflow(workflowId, { entry_agent_id: agentId })
            }
            onQuickCreateStarter={handleQuickCreateStarter}
            onAttachExistingAgent={(agentId) =>
              state.updateAgent(agentId, { workflow_id: activeWorkflowId })
            }
            onOpenCreate={openCreateDialog}
            onOpenSwarm={() => setSwarmOpen(true)}
          />
        </div>

        <div className="ca-pane ca-pane-inspector">
          {selected ? (
            <AgentInspector
              agent={selected}
              agents={state.agents}
              workflows={state.workflows}
              activeWorkflow={activeWorkflow}
              models={state.models}
              projects={workspace.projects}
              latestRun={latestRun}
              busy={state.busy}
              onSave={state.updateAgent}
              onDelete={state.deleteAgent}
              onProbe={state.probeAgent}
              onUpdateWorkflow={state.updateWorkflow}
              onClose={() => {
                setInspectorOpen(false)
                setMobilePane('workspace')
              }}
            />
          ) : (
            <aside className="ca-inspector ca-inspector-empty">
              <p className="ca-muted-note">{t('Выберите агента для настройки параметров.')}</p>
            </aside>
          )}
        </div>
      </div>

      {createOpen && (
        <CreateAgentDialog
          agents={state.agents}
          workflows={state.workflows}
          activeWorkflowId={activeWorkflowId}
          workspacePath={workspace.project?.kind === 'project' ? workspace.project.path : ''}
          pendingLink={pendingLink}
          onCreate={state.createAgent}
          onLinked={async (link, targetId) => {
            await state.updateAgent(
              link.sourceId,
              link.kind === 'next'
                ? { next_agent_id: targetId }
                : { verifier_agent_id: targetId }
            )
          }}
          onDispatchOrchestrator={async (orchestratorId, prompt) => {
            state.setSelectedAgentId(orchestratorId)
            setWorkspaceTab('tasks')
            setMobilePane('workspace')
            await state.runAgent({
              agentId: orchestratorId,
              prompt,
              triggerReason: 'create_via_orchestrator'
            })
          }}
          close={() => {
            setCreateOpen(false)
            setPendingLink(null)
          }}
        />
      )}

      {workflowDialogMode !== 'closed' && (
        <CreateWorkflowDialog
          workflow={
            workflowDialogMode === 'edit' ? (activeWorkflow as CustomWorkflow | null) : null
          }
          onCreate={state.createWorkflow}
          onUpdate={state.updateWorkflow}
          close={() => setWorkflowDialogMode('closed')}
        />
      )}

      {memoryOpen && (
        <GlobalMemoryDialog
          doc={state.globalMemory}
          onLoad={state.loadGlobalMemory}
          onSave={state.saveGlobalMemory}
          close={() => setMemoryOpen(false)}
        />
      )}

      {swarmOpen && (
        <SwarmDialog
          agents={state.agents}
          selectedAgent={selected}
          lastResult={lastSwarmResult}
          onDispatchSwarm={async (input) => {
            const res = await state.dispatchSwarm(input)
            setLastSwarmResult(res)
            return res
          }}
          onShowBoard={(chainId) => {
            state.setSelectedChainId(chainId)
            setWorkspaceTab('runs')
            setRequestedRunsSubTab('board')
            setMobilePane('workspace')
          }}
          close={() => setSwarmOpen(false)}
        />
      )}
    </div>
  )
}

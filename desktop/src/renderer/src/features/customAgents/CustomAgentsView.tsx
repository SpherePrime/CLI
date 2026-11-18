import { useState } from 'react'
import {
  Activity,
  Bot,
  Brain,
  GitBranch,
  History,
  Layers,
  PanelLeft,
  Plus,
  RefreshCw,
  SlidersHorizontal,
  Users
} from 'lucide-react'
import type { SwarmResult } from '../../../../shared/customAgents'
import type { Workspace } from '../../app/useWorkspace'
import { EmptyState } from '../../components/EmptyState'
import { useI18n } from '../../i18n'
import { AgentInspector } from './AgentInspector'
import { AgentRoster } from './AgentRoster'
import {
  AgentWorkspace,
  type RunsSubTab,
  type WorkspaceTab
} from './AgentWorkspace'
import { CreateAgentDialog } from './CreateAgentDialog'
import { GlobalMemoryDialog } from './GlobalMemoryDialog'
import type { RoleFilter } from './presentation'
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

  const openCreateDialog = (link?: PendingWorkflowLink) => {
    setPendingLink(link || null)
    setCreateOpen(true)
  }

  const openSettingsForAgent = (agentId: string) => {
    state.setSelectedAgentId(agentId)
    setInspectorOpen(true)
    setMobilePane('inspector')
  }

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
            onOpenCreate={() => openCreateDialog()}
          />
        </div>

        <div className="ca-pane ca-pane-workspace">
          {selected ? (
            <AgentWorkspace
              agent={selected}
              agents={state.agents}
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
              onOpenCreate={openCreateDialog}
              onOpenSwarm={() => setSwarmOpen(true)}
            />
          ) : (
            <div className="ca-workspace-empty">
              <EmptyState
                icon={Bot}
                title={t('Выберите или создайте агента')}
                text={t(
                  'Настройте автономного исполнителя, мониторинг изменений или координатора цепочек.'
                )}
                action={{
                  label: t('Новый агент'),
                  icon: Plus,
                  onClick: () => openCreateDialog()
                }}
              />
            </div>
          )}
        </div>

        <div className="ca-pane ca-pane-inspector">
          {selected ? (
            <AgentInspector
              agent={selected}
              agents={state.agents}
              models={state.models}
              projects={workspace.projects}
              latestRun={latestRun}
              busy={state.busy}
              onSave={state.updateAgent}
              onDelete={state.deleteAgent}
              onProbe={state.probeAgent}
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

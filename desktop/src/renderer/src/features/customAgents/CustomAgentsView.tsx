import { useState } from 'react'
import { Bot, Brain, Plus, RefreshCw, SlidersHorizontal, Users } from 'lucide-react'
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
  const [inspectorOpen, setInspectorOpen] = useState(true)
  const [requestedTab, setRequestedTab] = useState<WorkspaceTab | undefined>(undefined)
  const [requestedRunsSubTab, setRequestedRunsSubTab] = useState<RunsSubTab | undefined>(undefined)
  const [mobilePane, setMobilePane] = useState<MobilePane>('workspace')

  const selected = state.selectedAgent
  const latestRun = state.agentRuns[0] || null

  const openCreateDialog = (link?: PendingWorkflowLink) => {
    setPendingLink(link || null)
    setCreateOpen(true)
  }

  return (
    <div className="ca-studio" data-mobile-pane={mobilePane} aria-label={t('Свои агенты')}>
      <header className="ca-studio-head">
        <div className="ca-studio-head-title">
          <h1>{t('Студия агентов')}</h1>
          {state.globalMemory && (
            <span className="ca-studio-mem-pill">
              {`MEMORY ${state.globalMemory.memory_chars}/${state.globalMemory.memory_limit}`}
            </span>
          )}
        </div>
        <div className="ca-studio-head-actions">
          <button
            type="button"
            className="ca-memory-trigger"
            onClick={() => setMemoryOpen(true)}
          >
            <Brain size={14} aria-hidden="true" />
            <span>{t('Память и личность')}</span>
          </button>
          <button
            type="button"
            className={`ca-inspector-toggle ${inspectorOpen ? 'active' : ''}`}
            aria-pressed={inspectorOpen}
            onClick={() => setInspectorOpen((prev) => !prev)}
          >
            <SlidersHorizontal size={14} aria-hidden="true" />
            <span>{t('Параметры')}</span>
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
          <span>{t('Инспектор')}</span>
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

      <div className={`ca-grid ${inspectorOpen ? '' : 'inspector-collapsed'}`}>
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
              requestedTab={requestedTab}
              requestedRunsSubTab={requestedRunsSubTab}
              onSelectChainId={state.setSelectedChainId}
              onSelectAgent={state.setSelectedAgentId}
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

        {inspectorOpen && (
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
                onClose={() => setInspectorOpen(false)}
              />
            ) : (
              <aside className="ca-inspector ca-inspector-empty">
                <p className="ca-muted-note">{t('Выберите агента для настройки параметров.')}</p>
              </aside>
            )}
          </div>
        )}
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
            setRequestedTab('runs')
            setRequestedRunsSubTab('board')
          }}
          close={() => setSwarmOpen(false)}
        />
      )}
    </div>
  )
}

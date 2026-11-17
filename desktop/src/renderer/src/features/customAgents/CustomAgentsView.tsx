import { useState } from 'react'
import { Bot, Plus, RefreshCw, SlidersHorizontal, Users } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import { EmptyState } from '../../components/EmptyState'
import { useI18n } from '../../i18n'
import { AgentInspector } from './AgentInspector'
import { AgentRoster } from './AgentRoster'
import { AgentWorkspace } from './AgentWorkspace'
import { CreateAgentDialog } from './CreateAgentDialog'
import type { RoleFilter } from './presentation'
import { useCustomAgents } from './useCustomAgents'
import './customAgents.css'

type MobilePane = 'roster' | 'workspace' | 'inspector'

export function CustomAgentsView({ workspace }: { workspace: Workspace }) {
  const { t } = useI18n()
  const state = useCustomAgents()
  const [query, setQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState<RoleFilter>('all')
  const [createOpen, setCreateOpen] = useState(false)
  const [mobilePane, setMobilePane] = useState<MobilePane>('workspace')

  const selected = state.selectedAgent
  const latestRun = state.agentRuns[0] || null

  return (
    <div className="ca-studio" data-mobile-pane={mobilePane} aria-label={t('Свои агенты')}>
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
          onClick={() => setMobilePane('inspector')}
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

      <div className="ca-grid">
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
            onOpenCreate={() => setCreateOpen(true)}
          />
        </div>

        <div className="ca-pane ca-pane-workspace">
          {selected ? (
            <AgentWorkspace
              agent={selected}
              agents={state.agents}
              runs={state.agentRuns}
              allRuns={state.runs}
              blackboard={state.blackboard}
              selectedChainId={state.selectedChainId}
              onSelectChainId={state.setSelectedChainId}
              onSelectAgent={state.setSelectedAgentId}
              onRun={state.runAgent}
              onCancelRun={state.cancelRun}
              onProbe={state.probeAgent}
              onPostBlackboard={state.postBlackboard}
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
                  onClick: () => setCreateOpen(true)
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
          onCreate={state.createAgent}
          onDispatchOrchestrator={async (orchestratorId, prompt) => {
            state.setSelectedAgentId(orchestratorId)
            setMobilePane('workspace')
            await state.runAgent({
              agentId: orchestratorId,
              prompt,
              triggerReason: 'create_via_orchestrator'
            })
          }}
          close={() => setCreateOpen(false)}
        />
      )}
    </div>
  )
}

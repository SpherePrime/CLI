import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Bot,
  Crosshair,
  Layers,
  LayoutGrid,
  Minus,
  Play,
  Plus,
  ShieldCheck,
  Sparkles,
  TriangleAlert,
  X
} from 'lucide-react'
import type {
  CustomAgent,
  ModelOption,
  UpdateCustomAgentInput
} from '../../../../shared/customAgents'
import { useI18n } from '../../i18n'
import {
  agentStatusMeta,
  modelSummary,
  triggerLabel
} from './presentation'
import {
  buildBezierPath,
  canConnectAgents,
  computeAutoLayout,
  getPortPoint,
  loadSavedLayout,
  mergeNodePositions,
  saveLayout,
  WORKFLOW_NODE_HEIGHT,
  WORKFLOW_NODE_WIDTH,
  type NodePosition,
  type WorkflowLinkKind
} from './workflowLayout'

export interface PendingWorkflowLink {
  sourceId: string
  kind: WorkflowLinkKind
}

interface WorkflowEdge {
  id: string
  sourceId: string
  targetId: string
  kind: WorkflowLinkKind
  x1: number
  y1: number
  x2: number
  y2: number
  path: string
  midX: number
  midY: number
}

const PORT_MENU_WIDTH = 224
const PORT_MENU_ESTIMATED_HEIGHT = 248

export function WorkflowCanvas({
  agents,
  models,
  selectedAgentId,
  onSelectAgent,
  onRunAgent,
  onUpdateAgent,
  onOpenCreate,
  onOpenSwarm
}: {
  agents: CustomAgent[]
  models: ModelOption[]
  selectedAgentId: string
  onSelectAgent: (id: string) => void
  onRunAgent: (agentId: string) => void
  onUpdateAgent: (agentId: string, patch: UpdateCustomAgentInput) => Promise<CustomAgent>
  onOpenCreate: (pendingLink?: PendingWorkflowLink) => void
  onOpenSwarm: () => void
}) {
  const { t } = useI18n()
  const stageRef = useRef<HTMLDivElement>(null)
  const [positions, setPositions] = useState<Record<string, NodePosition>>(() =>
    mergeNodePositions(agents, loadSavedLayout())
  )
  const [viewport, setViewport] = useState({ x: 24, y: 16, zoom: 1 })
  const [dragNode, setDragNode] = useState<{
    id: string
    startX: number
    startY: number
    origX: number
    origY: number
    moved: boolean
  } | null>(null)
  const [panState, setPanState] = useState<{
    startX: number
    startY: number
    origX: number
    origY: number
  } | null>(null)
  const [wireDraft, setWireDraft] = useState<{
    sourceId: string
    kind: WorkflowLinkKind
    startClientX: number
    startClientY: number
    x1: number
    y1: number
    x2: number
    y2: number
    moved: boolean
  } | null>(null)
  const [portMenu, setPortMenu] = useState<{
    sourceId: string
    kind: WorkflowLinkKind
    x: number
    y: number
  } | null>(null)
  const [selectedEdge, setSelectedEdge] = useState<{
    sourceId: string
    targetId: string
    kind: WorkflowLinkKind
  } | null>(null)
  const [wireError, setWireError] = useState('')

  useEffect(() => {
    setPositions((current) => mergeNodePositions(agents, current))
  }, [agents])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setPortMenu(null)
        setSelectedEdge(null)
        setWireDraft(null)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  const toCanvasCoords = (clientX: number, clientY: number): NodePosition => {
    const rect = stageRef.current?.getBoundingClientRect()
    if (!rect) return { x: 0, y: 0 }
    return {
      x: (clientX - rect.left - viewport.x) / viewport.zoom,
      y: (clientY - rect.top - viewport.y) / viewport.zoom
    }
  }

  const openPortMenu = (
    sourceId: string,
    kind: WorkflowLinkKind,
    nodePos: NodePosition
  ) => {
    const anchorPt = getPortPoint(nodePos, kind)
    const vx = anchorPt.x * viewport.zoom + viewport.x
    const vy = anchorPt.y * viewport.zoom + viewport.y
    const rect = stageRef.current?.getBoundingClientRect()
    const stageW = rect?.width || 640
    const stageH = rect?.height || 480
    const preferRight = vx + 12 + PORT_MENU_WIDTH <= stageW - 12
    const rawX = preferRight ? vx + 12 : vx - PORT_MENU_WIDTH - 12
    const clampedX = Math.round(
      Math.max(12, Math.min(rawX, Math.max(12, stageW - PORT_MENU_WIDTH - 12)))
    )
    const clampedY = Math.round(
      Math.max(12, Math.min(vy - 18, Math.max(12, stageH - PORT_MENU_ESTIMATED_HEIGHT - 12)))
    )
    setWireError('')
    setSelectedEdge(null)
    setPortMenu({ sourceId, kind, x: clampedX, y: clampedY })
  }

  useEffect(() => {
    if (!dragNode && !panState && !wireDraft) return

    const handleMove = (event: PointerEvent) => {
      if (dragNode) {
        const dx = (event.clientX - dragNode.startX) / viewport.zoom
        const dy = (event.clientY - dragNode.startY) / viewport.zoom
        const moved =
          dragNode.moved ||
          Math.abs(event.clientX - dragNode.startX) > 4 ||
          Math.abs(event.clientY - dragNode.startY) > 4
        if (moved && !dragNode.moved) {
          setDragNode((prev) => (prev ? { ...prev, moved: true } : null))
        }
        setPositions((current) => ({
          ...current,
          [dragNode.id]: {
            x: Math.round(dragNode.origX + dx),
            y: Math.round(dragNode.origY + dy)
          }
        }))
        return
      }

      if (panState) {
        setViewport((current) => ({
          ...current,
          x: Math.round(panState.origX + (event.clientX - panState.startX)),
          y: Math.round(panState.origY + (event.clientY - panState.startY))
        }))
        return
      }

      if (wireDraft) {
        const pt = toCanvasCoords(event.clientX, event.clientY)
        const moved =
          wireDraft.moved ||
          Math.hypot(
            event.clientX - wireDraft.startClientX,
            event.clientY - wireDraft.startClientY
          ) > 5
        setWireDraft((prev) =>
          prev ? { ...prev, x2: pt.x, y2: pt.y, moved } : null
        )
      }
    }

    const handleUp = (event: PointerEvent) => {
      if (dragNode) {
        if (dragNode.moved) {
          setPositions((current) => {
            saveLayout(current)
            return current
          })
        } else {
          onSelectAgent(dragNode.id)
        }
        setDragNode(null)
      }

      if (panState) {
        setPanState(null)
      }

      if (wireDraft) {
        if (!wireDraft.moved) {
          const sourcePos = positions[wireDraft.sourceId] || { x: 52, y: 72 }
          openPortMenu(wireDraft.sourceId, wireDraft.kind, sourcePos)
        } else {
          const hit = document.elementFromPoint(event.clientX, event.clientY)
          const targetEl = hit?.closest('[data-wf-in-node],[data-wf-node-id]')
          const targetId =
            targetEl?.getAttribute('data-wf-in-node') ||
            targetEl?.getAttribute('data-wf-node-id') ||
            ''
          if (targetId) {
            void commitConnection(wireDraft.sourceId, targetId, wireDraft.kind)
          }
        }
        setWireDraft(null)
      }
    }

    window.addEventListener('pointermove', handleMove)
    window.addEventListener('pointerup', handleUp)
    return () => {
      window.removeEventListener('pointermove', handleMove)
      window.removeEventListener('pointerup', handleUp)
    }
  })

  const commitConnection = async (
    sourceId: string,
    targetId: string,
    kind: WorkflowLinkKind
  ) => {
    const check = canConnectAgents(agents, sourceId, targetId, kind)
    if (!check.ok) {
      setWireError(t(check.reason || 'Невозможно создать связь'))
      return
    }
    setWireError('')
    setPortMenu(null)
    setSelectedEdge(null)
    try {
      await onUpdateAgent(
        sourceId,
        kind === 'next' ? { next_agent_id: targetId } : { verifier_agent_id: targetId }
      )
    } catch (cause) {
      setWireError(String((cause as Error)?.message || cause))
    }
  }

  const removeConnection = async (
    sourceId: string,
    kind: WorkflowLinkKind
  ) => {
    setWireError('')
    setSelectedEdge(null)
    try {
      await onUpdateAgent(
        sourceId,
        kind === 'next' ? { next_agent_id: '' } : { verifier_agent_id: '' }
      )
    } catch (cause) {
      setWireError(String((cause as Error)?.message || cause))
    }
  }

  const edges = useMemo<WorkflowEdge[]>(() => {
    const byId = new Map(agents.map((agent) => [agent.id, agent]))
    const list: WorkflowEdge[] = []
    for (const agent of agents) {
      const fromPos = positions[agent.id]
      if (!fromPos) continue
      if (agent.next_agent_id && byId.has(agent.next_agent_id)) {
        const toPos = positions[agent.next_agent_id]
        if (toPos) {
          const p1 = getPortPoint(fromPos, 'next')
          const p2 = getPortPoint(toPos, 'in')
          const curve = buildBezierPath(p1.x, p1.y, p2.x, p2.y)
          list.push({
            id: `${agent.id}:next:${agent.next_agent_id}`,
            sourceId: agent.id,
            targetId: agent.next_agent_id,
            kind: 'next',
            x1: p1.x,
            y1: p1.y,
            x2: p2.x,
            y2: p2.y,
            ...curve
          })
        }
      }
      if (agent.verifier_agent_id && byId.has(agent.verifier_agent_id)) {
        const toPos = positions[agent.verifier_agent_id]
        if (toPos) {
          const p1 = getPortPoint(fromPos, 'verifier')
          const p2 = getPortPoint(toPos, 'in')
          const curve = buildBezierPath(p1.x, p1.y, p2.x, p2.y)
          list.push({
            id: `${agent.id}:verifier:${agent.verifier_agent_id}`,
            sourceId: agent.id,
            targetId: agent.verifier_agent_id,
            kind: 'verifier',
            x1: p1.x,
            y1: p1.y,
            x2: p2.x,
            y2: p2.y,
            ...curve
          })
        }
      }
    }
    return list
  }, [agents, positions])

  const activeEdgeOverlay = useMemo(() => {
    if (!selectedEdge) return null
    return (
      edges.find(
        (edge) =>
          edge.sourceId === selectedEdge.sourceId &&
          edge.targetId === selectedEdge.targetId &&
          edge.kind === selectedEdge.kind
      ) || null
    )
  }, [edges, selectedEdge])

  const handleAutoLayout = () => {
    const next = computeAutoLayout(agents)
    setPositions(next)
    saveLayout(next)
    setViewport({ x: 24, y: 16, zoom: 1 })
    setPortMenu(null)
    setSelectedEdge(null)
  }

  const handleCenterView = () => {
    const rect = stageRef.current?.getBoundingClientRect()
    const coords = agents
      .map((agent) => positions[agent.id])
      .filter((pos): pos is NodePosition => Boolean(pos))
    if (!rect || coords.length === 0) {
      setViewport({ x: 24, y: 16, zoom: 1 })
      return
    }
    const minX = Math.min(...coords.map((p) => p.x))
    const minY = Math.min(...coords.map((p) => p.y))
    const maxX = Math.max(...coords.map((p) => p.x + WORKFLOW_NODE_WIDTH))
    const maxY = Math.max(...coords.map((p) => p.y + WORKFLOW_NODE_HEIGHT))
    const contentW = Math.max(260, maxX - minX)
    const contentH = Math.max(180, maxY - minY)
    const pad = 48
    const fitZoom = Math.min(
      1.15,
      Math.max(
        0.5,
        Math.min(
          (rect.width - pad * 2) / contentW,
          (rect.height - pad * 2) / contentH
        )
      )
    )
    const offsetX = Math.round((rect.width - contentW * fitZoom) / 2 - minX * fitZoom)
    const offsetY = Math.round((rect.height - contentH * fitZoom) / 2 - minY * fitZoom)
    setViewport({ x: offsetX, y: offsetY, zoom: Number(fitZoom.toFixed(2)) })
  }

  const setZoom = (delta: number) => {
    setViewport((current) => ({
      ...current,
      zoom: Number(Math.min(1.6, Math.max(0.45, current.zoom + delta)).toFixed(2))
    }))
  }

  const roleIcon = (role: string) => {
    if (role === 'orchestrator') return <Sparkles size={14} aria-hidden="true" />
    if (role === 'verifier') return <ShieldCheck size={14} aria-hidden="true" />
    return <Bot size={14} aria-hidden="true" />
  }

  const qualityBadge = (agent: CustomAgent) => {
    if (agent.strict_verify) return t('Строгий контроль')
    if (
      agent.acceptance_criteria.trim() ||
      (agent.deterministic_checks && agent.deterministic_checks.length > 0)
    ) {
      return t('Критерии')
    }
    return t('Без проверки')
  }

  const portMenuTargets = useMemo(() => {
    if (!portMenu) return []
    return agents.filter(
      (candidate) =>
        canConnectAgents(agents, portMenu.sourceId, candidate.id, portMenu.kind).ok
    )
  }, [agents, portMenu])

  return (
    <div className="ca-wf-viewport">
      <div className="ca-wf-toolbar">
        <button
          type="button"
          className="primary ca-wf-tool-btn"
          onClick={() => onOpenCreate()}
        >
          <Plus size={14} aria-hidden="true" />
          <span>{t('Новый узел')}</span>
        </button>
        <button
          type="button"
          className="ca-wf-tool-btn"
          onClick={onOpenSwarm}
        >
          <Layers size={14} aria-hidden="true" />
          <span>{t('Запустить рой (Swarm)')}</span>
        </button>
      </div>

      <div
        ref={stageRef}
        className={`ca-wf-stage ${panState ? 'panning' : ''}`}
        onPointerDown={(event) => {
          const target = event.target as HTMLElement
          if (
            target.closest(
              '.ca-wf-node, .ca-wf-zoom-bar, .ca-wf-port-menu, .ca-wf-edge-chip, .ca-wf-edge-hit, .ca-wf-error'
            )
          ) {
            return
          }
          setPortMenu(null)
          setSelectedEdge(null)
          setPanState({
            startX: event.clientX,
            startY: event.clientY,
            origX: viewport.x,
            origY: viewport.y
          })
        }}
      >
        <div className="ca-wf-zoom-bar" role="group" aria-label={t('Управление холстом')}>
          <button
            type="button"
            className="icon small"
            title={t('Уменьшить')}
            aria-label={t('Уменьшить')}
            onClick={() => setZoom(-0.15)}
          >
            <Minus size={13} aria-hidden="true" />
          </button>
          <button
            type="button"
            className="ca-wf-zoom-readout"
            title={t('Сбросить масштаб')}
            onClick={() => setViewport((current) => ({ ...current, zoom: 1 }))}
          >
            {`${Math.round(viewport.zoom * 100)}%`}
          </button>
          <button
            type="button"
            className="icon small"
            title={t('Увеличить')}
            aria-label={t('Увеличить')}
            onClick={() => setZoom(0.15)}
          >
            <Plus size={13} aria-hidden="true" />
          </button>
          <button
            type="button"
            className="ca-wf-mini-action"
            title={t('По центру')}
            onClick={handleCenterView}
          >
            <Crosshair size={13} aria-hidden="true" />
            <span>{t('По центру')}</span>
          </button>
          <button
            type="button"
            className="ca-wf-mini-action"
            title={t('Авто-раскладка')}
            onClick={handleAutoLayout}
          >
            <LayoutGrid size={13} aria-hidden="true" />
            <span>{t('Авто-раскладка')}</span>
          </button>
        </div>

        {wireError && (
          <div className="ca-wf-error" role="alert">
            <TriangleAlert size={14} aria-hidden="true" />
            <span>{wireError}</span>
            <button
              type="button"
              className="icon small"
              aria-label={t('Закрыть')}
              onClick={() => setWireError('')}
            >
              <X size={12} aria-hidden="true" />
            </button>
          </div>
        )}

        <div
          className="ca-wf-world"
          style={{
            transform: `translate(${viewport.x}px, ${viewport.y}px) scale(${viewport.zoom})`
          }}
        >
          <svg className="ca-wf-svg" aria-hidden="true">
            <defs>
              <marker
                id="ca-arrow-next"
                markerWidth="8"
                markerHeight="8"
                refX="6"
                refY="4"
                orient="auto"
              >
                <path d="M 0 1 L 7 4 L 0 7 z" className="ca-wf-arrow-next" />
              </marker>
              <marker
                id="ca-arrow-verifier"
                markerWidth="8"
                markerHeight="8"
                refX="6"
                refY="4"
                orient="auto"
              >
                <path d="M 0 1 L 7 4 L 0 7 z" className="ca-wf-arrow-verifier" />
              </marker>
            </defs>

            {edges.map((edge) => {
              const isSelected =
                selectedEdge?.sourceId === edge.sourceId &&
                selectedEdge?.targetId === edge.targetId &&
                selectedEdge?.kind === edge.kind
              const isRelated =
                edge.sourceId === selectedAgentId || edge.targetId === selectedAgentId
              return (
                <g key={edge.id}>
                  <path
                    d={edge.path}
                    className="ca-wf-edge-hit"
                    onClick={(event) => {
                      event.stopPropagation()
                      setPortMenu(null)
                      setSelectedEdge({
                        sourceId: edge.sourceId,
                        targetId: edge.targetId,
                        kind: edge.kind
                      })
                    }}
                  />
                  <path
                    d={edge.path}
                    className={`ca-wf-edge kind-${edge.kind} ${isRelated ? 'related' : ''} ${isSelected ? 'selected' : ''}`}
                    markerEnd={
                      edge.kind === 'next'
                        ? 'url(#ca-arrow-next)'
                        : 'url(#ca-arrow-verifier)'
                    }
                  />
                </g>
              )
            })}

            {wireDraft && wireDraft.moved && (
              <path
                d={
                  buildBezierPath(
                    wireDraft.x1,
                    wireDraft.y1,
                    wireDraft.x2,
                    wireDraft.y2
                  ).path
                }
                className={`ca-wf-edge draft kind-${wireDraft.kind}`}
              />
            )}
          </svg>

          {activeEdgeOverlay && (
            <div
              className="ca-wf-edge-chip"
              style={{
                left: `${activeEdgeOverlay.midX}px`,
                top: `${activeEdgeOverlay.midY}px`
              }}
              onClick={(event) => event.stopPropagation()}
            >
              <span>
                {activeEdgeOverlay.kind === 'next' ? t('Далее') : t('Проверка')}
              </span>
              <button
                type="button"
                className="danger ca-wf-edge-del"
                onClick={(event) => {
                  event.stopPropagation()
                  void removeConnection(
                    activeEdgeOverlay.sourceId,
                    activeEdgeOverlay.kind
                  )
                }}
              >
                <X size={12} aria-hidden="true" />
                <span>{t('Удалить связь')}</span>
              </button>
            </div>
          )}

          {agents.map((agent) => {
            const pos = positions[agent.id] || { x: 52, y: 72 }
            const selected = agent.id === selectedAgentId
            const status = agentStatusMeta(agent.status)
            const model = modelSummary(agent, models)
            const hasTrigger = agent.trigger_type && agent.trigger_type !== 'manual'

            return (
              <article
                key={agent.id}
                data-wf-node-id={agent.id}
                className={`ca-wf-node role-${agent.role} ${selected ? 'selected' : ''} ${agent.enabled ? '' : 'disabled'}`}
                style={{
                  transform: `translate(${pos.x}px, ${pos.y}px)`,
                  width: `${WORKFLOW_NODE_WIDTH}px`
                }}
                onClick={() => {
                  setPortMenu(null)
                  setSelectedEdge(null)
                  onSelectAgent(agent.id)
                }}
              >
                <div className="ca-wf-node-accent" />

                <button
                  type="button"
                  data-wf-in-node={agent.id}
                  className="ca-wf-port ca-wf-port-in"
                  title={t('Вход узла')}
                  aria-label={t('Вход узла {name}', { name: agent.name })}
                  onClick={(event) => event.stopPropagation()}
                >
                  <span className="ca-wf-port-dot ca-wf-port-dot-in" />
                </button>

                <header
                  className="ca-wf-node-head"
                  onPointerDown={(event) => {
                    if ((event.target as HTMLElement).closest('button')) return
                    event.stopPropagation()
                    setPortMenu(null)
                    setSelectedEdge(null)
                    setDragNode({
                      id: agent.id,
                      startX: event.clientX,
                      startY: event.clientY,
                      origX: pos.x,
                      origY: pos.y,
                      moved: false
                    })
                  }}
                >
                  <span className="ca-wf-role-icon">{roleIcon(agent.role)}</span>
                  <div className="ca-wf-node-titles">
                    <strong title={agent.name}>{agent.name}</strong>
                    <span>{`@${agent.slug}`}</span>
                  </div>
                  <button
                    type="button"
                    className="ca-wf-run-btn"
                    title={t('Запустить {name}', { name: agent.name })}
                    aria-label={t('Запустить {name}', { name: agent.name })}
                    onClick={(event) => {
                      event.stopPropagation()
                      onSelectAgent(agent.id)
                      onRunAgent(agent.id)
                    }}
                  >
                    <Play size={12} aria-hidden="true" />
                  </button>
                </header>

                <div className="ca-wf-node-body">
                  <div className="ca-wf-node-row">
                    <span className={`ca-status-badge tone-${status.tone}`}>
                      <span className="ca-status-dot" aria-hidden="true" />
                      {t(status.label)}
                    </span>
                    <span
                      className={`ca-chip ${hasTrigger ? '' : 'ca-chip-muted'}`}
                      title={triggerLabel(agent)}
                    >
                      {triggerLabel(agent)}
                    </span>
                  </div>

                  <div className="ca-wf-node-row">
                    <span className="ca-chip ca-chip-muted" title={model}>
                      {model === 'Авто' ? t('Авто') : model}
                    </span>
                    <span className="ca-chip ca-chip-muted">
                      {qualityBadge(agent)}
                    </span>
                  </div>
                </div>

                <button
                  type="button"
                  className="ca-wf-port ca-wf-port-out port-next"
                  title={t('Выход: Далее')}
                  aria-label={t('Подключить следующий узел от {name}', {
                    name: agent.name
                  })}
                  onClick={(event) => {
                    event.stopPropagation()
                    if (event.detail === 0) {
                      openPortMenu(agent.id, 'next', pos)
                    }
                  }}
                  onPointerDown={(event) => {
                    event.stopPropagation()
                    const origin = getPortPoint(pos, 'next')
                    setSelectedEdge(null)
                    setWireDraft({
                      sourceId: agent.id,
                      kind: 'next',
                      startClientX: event.clientX,
                      startClientY: event.clientY,
                      x1: origin.x,
                      y1: origin.y,
                      x2: origin.x,
                      y2: origin.y,
                      moved: false
                    })
                  }}
                >
                  <span className="ca-wf-port-tag">{t('Далее')}</span>
                  <span className="ca-wf-port-dot" />
                </button>

                <button
                  type="button"
                  className="ca-wf-port ca-wf-port-out port-verifier"
                  title={t('Выход: Проверка')}
                  aria-label={t('Подключить верификатор к {name}', {
                    name: agent.name
                  })}
                  onClick={(event) => {
                    event.stopPropagation()
                    if (event.detail === 0) {
                      openPortMenu(agent.id, 'verifier', pos)
                    }
                  }}
                  onPointerDown={(event) => {
                    event.stopPropagation()
                    const origin = getPortPoint(pos, 'verifier')
                    setSelectedEdge(null)
                    setWireDraft({
                      sourceId: agent.id,
                      kind: 'verifier',
                      startClientX: event.clientX,
                      startClientY: event.clientY,
                      x1: origin.x,
                      y1: origin.y,
                      x2: origin.x,
                      y2: origin.y,
                      moved: false
                    })
                  }}
                >
                  <span className="ca-wf-port-tag">{t('Проверка')}</span>
                  <span className="ca-wf-port-dot" />
                </button>
              </article>
            )
          })}
        </div>

        {portMenu && (
          <div
            className="ca-wf-port-menu"
            style={{ left: `${portMenu.x}px`, top: `${portMenu.y}px` }}
            role="dialog"
            aria-label={t('Подключение узла')}
            onClick={(event) => event.stopPropagation()}
            onPointerDown={(event) => event.stopPropagation()}
          >
            <div className="ca-wf-port-menu-head">
              <strong>
                {portMenu.kind === 'next'
                  ? t('Подключить: Далее')
                  : t('Подключить: Верификатор')}
              </strong>
              <button
                type="button"
                className="icon small"
                aria-label={t('Закрыть')}
                onClick={() => setPortMenu(null)}
              >
                <X size={12} aria-hidden="true" />
              </button>
            </div>
            <div className="ca-wf-port-menu-list">
              {portMenuTargets.map((target) => (
                <button
                  key={target.id}
                  type="button"
                  className="ca-wf-port-menu-item"
                  onClick={() =>
                    void commitConnection(
                      portMenu.sourceId,
                      target.id,
                      portMenu.kind
                    )
                  }
                >
                  {roleIcon(target.role)}
                  <span>{target.name}</span>
                </button>
              ))}
              <button
                type="button"
                className="ca-wf-port-menu-create"
                onClick={() => {
                  const link: PendingWorkflowLink = {
                    sourceId: portMenu.sourceId,
                    kind: portMenu.kind
                  }
                  setPortMenu(null)
                  onOpenCreate(link)
                }}
              >
                <Plus size={13} aria-hidden="true" />
                <span>{t('Создать и подключить узел')}</span>
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

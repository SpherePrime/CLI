import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Bot,
  Crosshair,
  Flag,
  Layers,
  LayoutGrid,
  Minus,
  Play,
  Plus,
  Radar,
  Settings2,
  ShieldCheck,
  Sparkles,
  TriangleAlert,
  X
} from 'lucide-react'
import type {
  CustomAgent,
  CustomWorkflow,
  ModelOption,
  UpdateCustomAgentInput
} from '../../../../shared/customAgents'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import {
  AGENT_PRESETS,
  agentStatusMeta,
  modelSummary,
  triggerLabel,
  type AgentPreset
} from './presentation'
import {
  buildBezierPath,
  canConnectAgents,
  computeAutoLayout,
  computeFitViewport,
  getPortPoint,
  getScopedWorkflowAgents,
  loadSavedLayout,
  mergeNodePositions,
  parseLayoutJson,
  saveLayout,
  serializeLayoutJson,
  WORKFLOW_LAYOUT_STORAGE_KEY,
  type NodePosition,
  type ViewportState,
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

function workflowStorageKey(workflowId: string): string {
  return `${WORKFLOW_LAYOUT_STORAGE_KEY}:${workflowId || 'default'}`
}

export function WorkflowCanvas({
  agents,
  activeWorkflow = null,
  models,
  selectedAgentId,
  onSelectAgent,
  onConfigureAgent,
  onRunAgent,
  onUpdateAgent,
  onSaveLayout,
  onSetEntryAgent,
  onQuickCreateStarter,
  onAttachExistingAgent,
  onOpenCreate,
  onOpenSwarm
}: {
  agents: CustomAgent[]
  activeWorkflow?: CustomWorkflow | null
  models: ModelOption[]
  selectedAgentId: string
  onSelectAgent: (id: string) => void
  onConfigureAgent: (id: string) => void
  onRunAgent: (agentId: string) => void
  onUpdateAgent: (agentId: string, patch: UpdateCustomAgentInput) => Promise<CustomAgent>
  onSaveLayout?: (workflowId: string, layoutJson: string) => Promise<unknown>
  onSetEntryAgent?: (workflowId: string, agentId: string) => Promise<unknown>
  onQuickCreateStarter?: (preset: AgentPreset) => Promise<unknown>
  onAttachExistingAgent?: (agentId: string) => Promise<unknown>
  onOpenCreate: (pendingLink?: PendingWorkflowLink) => void
  onOpenSwarm: () => void
}) {
  const { t } = useI18n()
  const stageRef = useRef<HTMLDivElement>(null)
  const didFitWorkflowRef = useRef<string>('')
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const activeWorkflowId = activeWorkflow?.id || 'default'

  const { primaryIds, scopedAgents } = useMemo(
    () =>
      activeWorkflow
        ? getScopedWorkflowAgents(agents, activeWorkflowId)
        : {
            primaryIds: new Set(agents.map((item) => item.id)),
            scopedAgents: agents
          },
    [agents, activeWorkflow, activeWorkflowId]
  )

  const attachableAgents = useMemo(
    () => agents.filter((item) => !primaryIds.has(item.id)),
    [agents, primaryIds]
  )

  const computeInitialPositions = (
    targetAgents: CustomAgent[],
    wf: CustomWorkflow | null
  ): Record<string, NodePosition> => {
    const wfId = wf?.id || 'default'
    const fromServer = parseLayoutJson(wf?.layout_json)
    const fromLocal = loadSavedLayout(workflowStorageKey(wfId))
    const seed = Object.keys(fromServer).length > 0 ? { ...fromLocal, ...fromServer } : fromLocal
    return mergeNodePositions(targetAgents, seed)
  }

  const [positions, setPositions] = useState<Record<string, NodePosition>>(() =>
    computeInitialPositions(scopedAgents, activeWorkflow)
  )
  const [viewport, setViewport] = useState<ViewportState>({ x: 24, y: 20, zoom: 1 })
  const [gestureActive, setGestureActive] = useState(false)
  const [isPanning, setIsPanning] = useState(false)
  const [creatingStarter, setCreatingStarter] = useState('')
  const [attachValue, setAttachValue] = useState('')
  const [wirePreview, setWirePreview] = useState<{
    kind: WorkflowLinkKind
    x1: number
    y1: number
    x2: number
    y2: number
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

  const positionsRef = useRef(positions)
  positionsRef.current = positions
  const viewportRef = useRef(viewport)
  viewportRef.current = viewport
  const agentsRef = useRef(agents)
  agentsRef.current = agents
  const scopedAgentsRef = useRef(scopedAgents)
  scopedAgentsRef.current = scopedAgents
  const activeWorkflowRef = useRef(activeWorkflow)
  activeWorkflowRef.current = activeWorkflow

  const dragNodeRef = useRef<{
    id: string
    startX: number
    startY: number
    origX: number
    origY: number
    moved: boolean
  } | null>(null)
  const panStateRef = useRef<{
    startX: number
    startY: number
    origX: number
    origY: number
  } | null>(null)
  const wireDraftRef = useRef<{
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

  const selectedAgent = useMemo(
    () => scopedAgents.find((item) => item.id === selectedAgentId) || null,
    [scopedAgents, selectedAgentId]
  )

  const persistPositions = (
    next: Record<string, NodePosition>,
    immediate = false
  ) => {
    const wf = activeWorkflowRef.current
    const wfId = wf?.id || 'default'
    saveLayout(next, workflowStorageKey(wfId))
    if (!wf || !onSaveLayout) return
    const serialized = serializeLayoutJson(next)
    if (saveTimerRef.current) {
      clearTimeout(saveTimerRef.current)
      saveTimerRef.current = null
    }
    if (immediate) {
      void onSaveLayout(wf.id, serialized).catch(() => {})
      return
    }
    saveTimerRef.current = setTimeout(() => {
      saveTimerRef.current = null
      void onSaveLayout(wf.id, serialized).catch(() => {})
    }, 350)
  }

  useEffect(() => {
    return () => {
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    }
  }, [])

  useEffect(() => {
    const next = computeInitialPositions(scopedAgents, activeWorkflow)
    positionsRef.current = next
    setPositions(next)
    setPortMenu(null)
    setSelectedEdge(null)
    setWireError('')
    didFitWorkflowRef.current = ''
  }, [activeWorkflowId])

  useEffect(() => {
    const fromServer = parseLayoutJson(activeWorkflow?.layout_json)
    setPositions((current) => {
      const merged = mergeNodePositions(scopedAgents, { ...current, ...fromServer })
      positionsRef.current = merged
      return merged
    })
  }, [scopedAgents, activeWorkflow?.layout_json])

  useEffect(() => {
    const el = stageRef.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const tryFit = () => {
      if (
        didFitWorkflowRef.current === activeWorkflowId ||
        scopedAgentsRef.current.length === 0
      ) {
        return
      }
      const rect = el.getBoundingClientRect()
      if (rect.width > 40 && rect.height > 40) {
        didFitWorkflowRef.current = activeWorkflowId
        const fitted = computeFitViewport(positionsRef.current, rect.width, rect.height)
        viewportRef.current = fitted
        setViewport(fitted)
      }
    }
    tryFit()
    const observer = new ResizeObserver(tryFit)
    observer.observe(el)
    return () => observer.disconnect()
  }, [activeWorkflowId, scopedAgents.length])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setPortMenu(null)
        setSelectedEdge(null)
        wireDraftRef.current = null
        setWirePreview(null)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  const openPortMenu = (
    sourceId: string,
    kind: WorkflowLinkKind,
    nodePos: NodePosition
  ) => {
    const vp = viewportRef.current
    const anchorPt = getPortPoint(nodePos, kind)
    const vx = anchorPt.x * vp.zoom + vp.x
    const vy = anchorPt.y * vp.zoom + vp.y
    const rect = stageRef.current?.getBoundingClientRect()
    const stageW = rect?.width || 640
    const stageH = rect?.height || 480
    const preferRight = vx + 14 + PORT_MENU_WIDTH <= stageW - 12
    const rawX = preferRight ? vx + 14 : vx - PORT_MENU_WIDTH - 14
    const clampedX = Math.round(
      Math.max(12, Math.min(rawX, Math.max(12, stageW - PORT_MENU_WIDTH - 12)))
    )
    const clampedY = Math.round(
      Math.max(
        12,
        Math.min(vy - 20, Math.max(12, stageH - PORT_MENU_ESTIMATED_HEIGHT - 12))
      )
    )
    setWireError('')
    setSelectedEdge(null)
    setPortMenu({ sourceId, kind, x: clampedX, y: clampedY })
  }

  const commitConnection = async (
    sourceId: string,
    targetId: string,
    kind: WorkflowLinkKind
  ) => {
    const check = canConnectAgents(agentsRef.current, sourceId, targetId, kind)
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

  useEffect(() => {
    if (!gestureActive) return

    const handleMove = (event: PointerEvent) => {
      const drag = dragNodeRef.current
      if (drag) {
        const vp = viewportRef.current
        const dx = (event.clientX - drag.startX) / vp.zoom
        const dy = (event.clientY - drag.startY) / vp.zoom
        if (
          !drag.moved &&
          (Math.abs(event.clientX - drag.startX) > 4 ||
            Math.abs(event.clientY - drag.startY) > 4)
        ) {
          drag.moved = true
        }
        const nextPositions = {
          ...positionsRef.current,
          [drag.id]: {
            x: Math.round(drag.origX + dx),
            y: Math.round(drag.origY + dy)
          }
        }
        positionsRef.current = nextPositions
        setPositions(nextPositions)
        return
      }

      const pan = panStateRef.current
      if (pan) {
        const nextVp: ViewportState = {
          ...viewportRef.current,
          x: Math.round(pan.origX + (event.clientX - pan.startX)),
          y: Math.round(pan.origY + (event.clientY - pan.startY))
        }
        viewportRef.current = nextVp
        setViewport(nextVp)
        return
      }

      const wire = wireDraftRef.current
      if (wire) {
        const rect = stageRef.current?.getBoundingClientRect()
        const vp = viewportRef.current
        const pt = rect
          ? {
              x: (event.clientX - rect.left - vp.x) / vp.zoom,
              y: (event.clientY - rect.top - vp.y) / vp.zoom
            }
          : { x: wire.x2, y: wire.y2 }
        if (
          !wire.moved &&
          Math.hypot(
            event.clientX - wire.startClientX,
            event.clientY - wire.startClientY
          ) > 5
        ) {
          wire.moved = true
        }
        wire.x2 = pt.x
        wire.y2 = pt.y
        if (wire.moved) {
          setWirePreview({
            kind: wire.kind,
            x1: wire.x1,
            y1: wire.y1,
            x2: pt.x,
            y2: pt.y
          })
        }
      }
    }

    const handleUp = (event: PointerEvent) => {
      const drag = dragNodeRef.current
      if (drag) {
        if (drag.moved) {
          persistPositions(positionsRef.current, false)
        } else {
          onSelectAgent(drag.id)
        }
        dragNodeRef.current = null
      }

      if (panStateRef.current) {
        panStateRef.current = null
        setIsPanning(false)
      }

      const wire = wireDraftRef.current
      if (wire) {
        if (!wire.moved) {
          const sourcePos = positionsRef.current[wire.sourceId] || { x: 48, y: 64 }
          openPortMenu(wire.sourceId, wire.kind, sourcePos)
        } else {
          const hit = document.elementFromPoint(event.clientX, event.clientY)
          const targetEl = hit?.closest('[data-wf-in-node],[data-wf-node-id]')
          const targetId =
            targetEl?.getAttribute('data-wf-in-node') ||
            targetEl?.getAttribute('data-wf-node-id') ||
            ''
          if (targetId) {
            void commitConnection(wire.sourceId, targetId, wire.kind)
          }
        }
        wireDraftRef.current = null
        setWirePreview(null)
      }

      setGestureActive(false)
    }

    window.addEventListener('pointermove', handleMove)
    window.addEventListener('pointerup', handleUp)
    window.addEventListener('pointercancel', handleUp)
    return () => {
      window.removeEventListener('pointermove', handleMove)
      window.removeEventListener('pointerup', handleUp)
      window.removeEventListener('pointercancel', handleUp)
    }
  }, [gestureActive])

  const edges = useMemo<WorkflowEdge[]>(() => {
    const byId = new Map(scopedAgents.map((agent) => [agent.id, agent]))
    const list: WorkflowEdge[] = []
    let skipLaneIndex = 0
    for (const agent of scopedAgents) {
      if (!primaryIds.has(agent.id)) continue
      const fromPos = positions[agent.id]
      if (!fromPos) continue
      if (agent.next_agent_id && byId.has(agent.next_agent_id)) {
        const toPos = positions[agent.next_agent_id]
        if (toPos) {
          const p1 = getPortPoint(fromPos, 'next')
          const p2 = getPortPoint(toPos, 'in')
          const needsArc = p2.x <= p1.x + 24 || (p2.x - p1.x > 440 && Math.abs(p2.y - p1.y) < 128)
          const laneOffset = needsArc ? skipLaneIndex++ * 22 : 0
          const curve = buildBezierPath(p1.x, p1.y, p2.x, p2.y, { laneOffset })
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
          const needsArc = p2.x <= p1.x + 24 || (p2.x - p1.x > 440 && Math.abs(p2.y - p1.y) < 128)
          const laneOffset = needsArc ? skipLaneIndex++ * 22 : 0
          const curve = buildBezierPath(p1.x, p1.y, p2.x, p2.y, { laneOffset })
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
  }, [scopedAgents, primaryIds, positions])

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

  const fitCurrentNodes = (targetPositions: Record<string, NodePosition>) => {
    const rect = stageRef.current?.getBoundingClientRect()
    const fitted = computeFitViewport(
      targetPositions,
      rect?.width || 960,
      rect?.height || 560
    )
    viewportRef.current = fitted
    setViewport(fitted)
  }

  const handleAutoLayout = () => {
    const next = computeAutoLayout(scopedAgents)
    positionsRef.current = next
    setPositions(next)
    persistPositions(next, true)
    fitCurrentNodes(next)
    setPortMenu(null)
    setSelectedEdge(null)
  }

  const handleCenterView = () => {
    fitCurrentNodes(positionsRef.current)
  }

  const setZoom = (delta: number) => {
    setViewport((current) => {
      const next = {
        ...current,
        zoom: Number(Math.min(1.6, Math.max(0.45, current.zoom + delta)).toFixed(2))
      }
      viewportRef.current = next
      return next
    })
  }

  useEffect(() => {
    const stage = stageRef.current
    if (!stage) return
    const onWheel = (event: WheelEvent) => {
      const target = event.target as HTMLElement | null
      if (target?.closest('.ca-wf-port-menu, .ca-wf-empty-wrap')) return
      const rect = stage.getBoundingClientRect()
      if (event.ctrlKey || event.metaKey) {
        event.preventDefault()
        const vp = viewportRef.current
        const factor = event.deltaY < 0 ? 1.08 : 0.92
        const nextZoom = Number(Math.min(1.6, Math.max(0.45, vp.zoom * factor)).toFixed(2))
        const cursorX = event.clientX - rect.left
        const cursorY = event.clientY - rect.top
        const worldX = (cursorX - vp.x) / vp.zoom
        const worldY = (cursorY - vp.y) / vp.zoom
        const nextVp = {
          zoom: nextZoom,
          x: Math.round(cursorX - worldX * nextZoom),
          y: Math.round(cursorY - worldY * nextZoom)
        }
        viewportRef.current = nextVp
        setViewport(nextVp)
        return
      }
      event.preventDefault()
      const vp = viewportRef.current
      const nextVp = {
        ...vp,
        x: Math.round(vp.x - event.deltaX),
        y: Math.round(vp.y - event.deltaY)
      }
      viewportRef.current = nextVp
      setViewport(nextVp)
    }
    stage.addEventListener('wheel', onWheel, { passive: false })
    return () => stage.removeEventListener('wheel', onWheel)
  }, [])

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

  const startOutputPort = (
    event: React.PointerEvent,
    agentId: string,
    kind: WorkflowLinkKind,
    pos: NodePosition
  ) => {
    event.stopPropagation()
    const origin = getPortPoint(pos, kind)
    setSelectedEdge(null)
    wireDraftRef.current = {
      sourceId: agentId,
      kind,
      startClientX: event.clientX,
      startClientY: event.clientY,
      x1: origin.x,
      y1: origin.y,
      x2: origin.x,
      y2: origin.y,
      moved: false
    }
    setGestureActive(true)
  }

  const starterCards: { id: string; label: string; icon: typeof Radar }[] = [
    { id: 'monitor', label: '+ Монитор / Триггер', icon: Radar },
    { id: 'researcher', label: '+ Воркер', icon: Bot },
    { id: 'orchestrator', label: '+ Координатор', icon: Sparkles },
    { id: 'verifier', label: '+ Верификатор', icon: ShieldCheck }
  ]

  const handleStarter = async (presetId: string) => {
    const preset = AGENT_PRESETS.find((item) => item.id === presetId)
    if (!preset || !onQuickCreateStarter || creatingStarter) return
    setCreatingStarter(presetId)
    try {
      await onQuickCreateStarter(preset)
    } finally {
      setCreatingStarter('')
    }
  }

  const isEmptyWorkflow = scopedAgents.length === 0

  return (
    <div className="ca-wf-viewport">
      <div className="ca-wf-hud-top">
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

        {selectedAgent && (
          <div className="ca-wf-selected-pill">
            <strong title={selectedAgent.name}>{selectedAgent.name}</strong>
            {activeWorkflow && onSetEntryAgent && primaryIds.has(selectedAgent.id) && (
              <button
                type="button"
                className={`ca-wf-pill-action ${activeWorkflow.entry_agent_id === selectedAgent.id ? 'is-start' : ''}`}
                onClick={() =>
                  void onSetEntryAgent(
                    activeWorkflow.id,
                    activeWorkflow.entry_agent_id === selectedAgent.id
                      ? ''
                      : selectedAgent.id
                  )
                }
              >
                <Flag size={12} aria-hidden="true" />
                <span>
                  {activeWorkflow.entry_agent_id === selectedAgent.id
                    ? t('Старт')
                    : t('Сделать стартом')}
                </span>
              </button>
            )}
            <button
              type="button"
              className="ca-wf-pill-action primary"
              onClick={() => onRunAgent(selectedAgent.id)}
            >
              <Play size={12} aria-hidden="true" />
              <span>{t('Запустить')}</span>
            </button>
            <button
              type="button"
              className="ca-wf-pill-action"
              onClick={() => onConfigureAgent(selectedAgent.id)}
            >
              <Settings2 size={12} aria-hidden="true" />
              <span>{t('Настроить')}</span>
            </button>
          </div>
        )}

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
      </div>

      <div
        ref={stageRef}
        className={`ca-wf-stage ${isPanning ? 'panning' : ''}`}
        onPointerDown={(event) => {
          const target = event.target as HTMLElement
          if (
            target.closest(
              '.ca-wf-node, .ca-wf-zoom-bar, .ca-wf-port-menu, .ca-wf-edge-chip, .ca-wf-edge-hit, .ca-wf-error, .ca-wf-empty-hero'
            )
          ) {
            return
          }
          setPortMenu(null)
          setSelectedEdge(null)
          const vp = viewportRef.current
          panStateRef.current = {
            startX: event.clientX,
            startY: event.clientY,
            origX: vp.x,
            origY: vp.y
          }
          setIsPanning(true)
          setGestureActive(true)
        }}
      >
        {isEmptyWorkflow ? (
          <div className="ca-wf-empty-wrap">
            <div className="ca-wf-empty-hero">
              <h2>
                {t('Пустая цепочка «{name}» — выберите стартовый узел', {
                  name: activeWorkflow?.name || t('Новая цепочка')
                })}
              </h2>
              <p className="ca-muted-note">
                {t('Создайте первый узел в один клик или перенесите существующего агента.')}
              </p>
              <div className="ca-wf-starters-grid">
                {starterCards.map((item) => {
                  const Icon = item.icon
                  return (
                    <button
                      key={item.id}
                      type="button"
                      className="ca-wf-starter-btn"
                      disabled={Boolean(creatingStarter)}
                      onClick={() => void handleStarter(item.id)}
                    >
                      <Icon size={15} aria-hidden="true" />
                      <span>{t(item.label)}</span>
                    </button>
                  )
                })}
              </div>
              {attachableAgents.length > 0 && onAttachExistingAgent && (
                <div className="ca-wf-attach-row">
                  <span className="ca-muted-note">{t('Добавить существующего агента')}</span>
                  <Select
                    value={attachValue}
                    onChange={(value) => {
                      if (!value) return
                      setAttachValue('')
                      void onAttachExistingAgent(value)
                    }}
                    label={t('Добавить существующего агента')}
                    options={[
                      { value: '', label: t('Выберите агента…') },
                      ...attachableAgents.map((item) => ({
                        value: item.id,
                        label: item.name
                      }))
                    ]}
                  />
                </div>
              )}
            </div>
          </div>
        ) : (
          <>
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
                onClick={() => {
                  const next = { ...viewportRef.current, zoom: 1 }
                  viewportRef.current = next
                  setViewport(next)
                }}
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

                {wirePreview && (
                  <path
                    d={
                      buildBezierPath(
                        wirePreview.x1,
                        wirePreview.y1,
                        wirePreview.x2,
                        wirePreview.y2,
                        { skipArc: false }
                      ).path
                    }
                    className={`ca-wf-edge draft kind-${wirePreview.kind}`}
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

              {scopedAgents.map((agent) => {
                const pos = positions[agent.id] || { x: 48, y: 64 }
                const selected = agent.id === selectedAgentId
                const isEntry = activeWorkflow?.entry_agent_id === agent.id
                const isExternal = !primaryIds.has(agent.id)
                const status = agentStatusMeta(agent.status)
                const model = modelSummary(agent, models)
                const hasTrigger = agent.trigger_type && agent.trigger_type !== 'manual'

                return (
                  <article
                    key={agent.id}
                    data-wf-node-id={agent.id}
                    className={`ca-wf-node role-${agent.role} ${selected ? 'selected' : ''} ${agent.enabled ? '' : 'disabled'} ${isExternal ? 'external' : ''}`}
                    style={{
                      transform: `translate(${pos.x}px, ${pos.y}px)`
                    }}
                    onPointerDown={(event) => {
                      if (
                        (event.target as HTMLElement).closest(
                          'button, input, select, textarea, .ca-wf-port-anchor'
                        )
                      ) {
                        return
                      }
                      event.stopPropagation()
                      setPortMenu(null)
                      setSelectedEdge(null)
                      dragNodeRef.current = {
                        id: agent.id,
                        startX: event.clientX,
                        startY: event.clientY,
                        origX: pos.x,
                        origY: pos.y,
                        moved: false
                      }
                      setGestureActive(true)
                    }}
                    onDoubleClick={(event) => {
                      if (
                        (event.target as HTMLElement).closest(
                          'button, .ca-wf-port-anchor'
                        )
                      ) {
                        return
                      }
                      onSelectAgent(agent.id)
                      onConfigureAgent(agent.id)
                    }}
                    onClick={() => {
                      setPortMenu(null)
                      setSelectedEdge(null)
                      onSelectAgent(agent.id)
                    }}
                  >
                    <div className="ca-wf-node-accent" />

                    <div className="ca-wf-port-anchor anchor-in">
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
                    </div>

                    <header className="ca-wf-node-head">
                      <span className="ca-wf-role-icon">{roleIcon(agent.role)}</span>
                      <div className="ca-wf-node-titles">
                        <div className="ca-wf-node-title-row">
                          <strong title={agent.name}>{agent.name}</strong>
                          {isEntry && (
                            <span className="ca-wf-start-badge">{t('Старт')}</span>
                          )}
                        </div>
                        <span>{`@${agent.slug}`}</span>
                      </div>
                      <div className="ca-wf-node-actions">
                        <button
                          type="button"
                          className="ca-wf-node-btn"
                          title={t('Настроить {name}', { name: agent.name })}
                          aria-label={t('Настроить {name}', { name: agent.name })}
                          onClick={(event) => {
                            event.stopPropagation()
                            onSelectAgent(agent.id)
                            onConfigureAgent(agent.id)
                          }}
                        >
                          <Settings2 size={12} aria-hidden="true" />
                        </button>
                        <button
                          type="button"
                          className="ca-wf-node-btn ca-wf-run-btn"
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
                      </div>
                    </header>

                    <div className="ca-wf-node-body">
                      <div className="ca-wf-node-row">
                        <span className={`ca-status-badge tone-${status.tone}`}>
                          <span className="ca-status-dot" aria-hidden="true" />
                          {t(status.label)}
                        </span>
                        <span
                          className={`ca-chip ${hasTrigger ? '' : 'ca-chip-muted'}`}
                          title={triggerLabel(agent, t)}
                        >
                          {triggerLabel(agent, t)}
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

                    <div className="ca-wf-node-footer">
                      <div
                        className="ca-wf-port-row"
                        onClick={(event) => event.stopPropagation()}
                        onPointerDown={(event) =>
                          startOutputPort(event, agent.id, 'next', pos)
                        }
                      >
                        <span>{t('Далее')}</span>
                      </div>
                      <div
                        className="ca-wf-port-row"
                        onClick={(event) => event.stopPropagation()}
                        onPointerDown={(event) =>
                          startOutputPort(event, agent.id, 'verifier', pos)
                        }
                      >
                        <span>{t('Проверка')}</span>
                      </div>
                    </div>

                    <div className="ca-wf-port-anchor anchor-next">
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
                        onPointerDown={(event) =>
                          startOutputPort(event, agent.id, 'next', pos)
                        }
                      >
                        <span className="ca-wf-port-dot" />
                      </button>
                    </div>

                    <div className="ca-wf-port-anchor anchor-verifier">
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
                        onPointerDown={(event) =>
                          startOutputPort(event, agent.id, 'verifier', pos)
                        }
                      >
                        <span className="ca-wf-port-dot" />
                      </button>
                    </div>
                  </article>
                )
              })}
            </div>
          </>
        )}

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

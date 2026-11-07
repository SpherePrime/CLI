import type { AppUiNode, AppUiSnapshot, PickedAppElement } from '../../../../shared/appTesting'

export type AppUiRow = {
  ref: string
  depth: number
  label: string
  node: AppUiNode
}

export function flattenSnapshot(nodes: AppUiNode[]): AppUiRow[] {
  const rows: AppUiRow[] = []
  collectRows(nodes, 0, rows)
  return rows
}

function collectRows(nodes: AppUiNode[], depth: number, rows: AppUiRow[]): void {
  for (const node of nodes) {
    rows.push({ ref: node.ref, depth, label: nodeLabel(node), node })
    collectRows(node.children, depth + 1, rows)
  }
}

function nodeLabel(node: AppUiNode): string {
  const parts = [`${node.ref} ${node.role} "${singleLine(node.name)}" [${node.states.join(', ')}]`]
  const value = node.value ? singleLine(node.value) : ''
  if (value) {
    parts.push(`value="${value}"`)
  }
  parts.push(`(${node.bounds.x},${node.bounds.y},${node.bounds.width},${node.bounds.height})`)
  if (node.actions.length > 0) {
    parts.push(`actions=${node.actions.join(',')}`)
  }
  if (node.confidence !== 'high') {
    parts.push(`confidence=${node.confidence}`)
  }
  return parts.join(' ')
}

function singleLine(text: string): string {
  return text.replace(/\n/g, ' ')
}

export function pickedFromNode(node: AppUiNode, snapshot: AppUiSnapshot): PickedAppElement {
  return {
    element: {
      reference: node.ref,
      role: node.role,
      name: node.name,
      value: node.value ?? null,
      bounds: node.bounds,
      states: node.states,
      actions: node.actions,
      automationId: node.automationId ?? null,
      className: node.className ?? null,
      path: node.path,
      source: node.source,
      confidence: node.confidence
    },
    reference: node.ref,
    confidence: node.confidence,
    candidates: 1,
    window: snapshot.title
  }
}

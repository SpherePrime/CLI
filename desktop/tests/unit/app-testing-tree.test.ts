import { describe, expect, it } from 'vitest'
import { flattenSnapshot } from '../../src/renderer/src/features/appTesting/snapshotRows'
import type { AppUiNode } from '../../src/shared/appTesting'

function node(
  overrides: Partial<AppUiNode> & { ref: string; role: string; name: string }
): AppUiNode {
  return {
    bounds: { x: 0, y: 0, width: 0, height: 0 },
    states: [],
    actions: [],
    path: `${overrides.role}[${overrides.name}]`,
    source: 'uia',
    confidence: 'high',
    children: [],
    ...overrides
  }
}

function snapshotFixture(): AppUiNode[] {
  const window = node({
    ref: 'n1',
    role: 'window',
    name: 'Prime fixture',
    bounds: { x: 0, y: 0, width: 420, height: 260 },
    states: ['enabled']
  })
  const button = node({
    ref: 'n3',
    role: 'button',
    name: 'Save',
    bounds: { x: 120, y: 300, width: 50, height: 50 },
    states: ['enabled'],
    actions: ['invoke']
  })
  const edit = node({
    ref: 'n4',
    role: 'edit',
    name: 'Name',
    value: 'Иван',
    bounds: { x: 10, y: 10, width: 200, height: 24 },
    states: ['enabled'],
    actions: ['setValue']
  })
  window.children = [button, edit]
  return [window]
}

describe('flattenSnapshot', () => {
  it('returns an empty array for an empty snapshot', () => {
    expect(flattenSnapshot([])).toEqual([])
  })

  it('walks nested children with increasing depth', () => {
    const nodes = snapshotFixture()
    nodes[0].children[0].children = [
      node({ ref: 'n5', role: 'text', name: 'Hint', states: ['enabled'] })
    ]
    const rows = flattenSnapshot(nodes)
    expect(rows.map((row) => row.ref)).toEqual(['n1', 'n3', 'n5', 'n4'])
    expect(rows.map((row) => row.depth)).toEqual([0, 1, 2, 1])
  })

  it('renders role, name, states, bounds and actions for a button', () => {
    const rows = flattenSnapshot(snapshotFixture())
    expect(rows[1].label).toBe('n3 button "Save" [enabled] (120,300,50,50) actions=invoke')
  })

  it('renders the value of an edit before the bounds', () => {
    const rows = flattenSnapshot(snapshotFixture())
    expect(rows[2].label).toBe(
      'n4 edit "Name" [enabled] value="Иван" (10,10,200,24) actions=setValue'
    )
  })

  it('omits the value when it is empty and lists actions with commas', () => {
    const nodes = [
      node({
        ref: 'n1',
        role: 'list',
        name: 'Items',
        value: '',
        states: ['enabled', 'focusable'],
        actions: ['select', 'scroll']
      })
    ]
    const rows = flattenSnapshot(nodes)
    expect(rows[0].label).toBe(
      'n1 list "Items" [enabled, focusable] (0,0,0,0) actions=select,scroll'
    )
    expect(rows[0].label.includes('value=')).toBe(false)
  })

  it('renders low and medium confidence but not high', () => {
    const nodes = [
      node({ ref: 'n1', role: 'window', name: 'Prime fixture', confidence: 'high' }),
      node({ ref: 'n2', role: 'button', name: 'Save', confidence: 'low' }),
      node({ ref: 'n3', role: 'button', name: 'Cancel', confidence: 'medium' })
    ]
    const rows = flattenSnapshot(nodes)
    expect(rows[0].label.includes('confidence=')).toBe(false)
    expect(rows[1].label.endsWith('confidence=low')).toBe(true)
    expect(rows[2].label.endsWith('confidence=medium')).toBe(true)
  })
})

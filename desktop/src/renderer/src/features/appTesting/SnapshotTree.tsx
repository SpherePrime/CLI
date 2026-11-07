import { useMemo, useState } from 'react'
import { Search } from 'lucide-react'
import type { AppUiNode, AppUiSnapshot } from '../../../../shared/appTesting'
import type { AppTestingCopy } from './copy'
import { flattenSnapshot } from './snapshotRows'

const MAX_ROWS = 400

type Props = {
  copy: AppTestingCopy
  snapshot: AppUiSnapshot | null
  highlighted: string[]
  onPick(node: AppUiNode): void
}

export function SnapshotTree({ copy, snapshot, highlighted, onPick }: Props) {
  const [query, setQuery] = useState('')
  const rows = useMemo(() => flattenSnapshot(snapshot?.nodes ?? []), [snapshot])
  const needle = query.trim().toLowerCase()
  const visible = (needle ? rows.filter((row) => row.label.toLowerCase().includes(needle)) : rows).slice(
    0,
    MAX_ROWS
  )
  if (!snapshot) return null
  return (
    <div className="app-testing-tree">
      <label className="app-testing-search">
        <Search size={13} />
        <input
          value={query}
          placeholder={copy.search}
          aria-label={copy.search}
          onChange={(event) => setQuery(event.target.value)}
        />
      </label>
      {snapshot.truncated && <p className="app-testing-note">{copy.truncated}</p>}
      {snapshot.warnings.map((warning) => (
        <p className="app-testing-note" key={warning}>
          {warning}
        </p>
      ))}
      {visible.length === 0 ? (
        <p className="app-testing-note">{copy.noMatch}</p>
      ) : (
        <ul className="app-testing-rows" role="tree">
          {visible.map((row) => (
            <li key={row.ref} role="treeitem">
              <button
                type="button"
                className={highlighted.includes(row.ref) ? 'flagged' : ''}
                style={{ paddingLeft: `${6 + (needle ? 0 : Math.min(row.depth, 12) * 10)}px` }}
                title={row.label}
                onClick={() => onPick(row.node)}
              >
                <span className="app-testing-role">{row.node.role}</span>
                <span className="app-testing-name">{row.node.name || row.node.automationId || row.ref}</span>
                {row.node.value && <span className="app-testing-value">{row.node.value}</span>}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

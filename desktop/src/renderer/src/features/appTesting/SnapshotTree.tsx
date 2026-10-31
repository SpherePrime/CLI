import { useMemo } from 'react'
import type { AppUiSnapshot } from '../../../../shared/appTesting'
import { appTestingCopy } from './copy'
import { flattenSnapshot } from './snapshotRows'

type Props = {
  language: 'ru' | 'en'
  snapshot?: AppUiSnapshot | null
  query: string
  selected?: string
  onSelect(ref: string): void
}

export function SnapshotTree({ language, snapshot, query, selected, onSelect }: Props) {
  const c = appTestingCopy[language]
  const rows = useMemo(() => flattenSnapshot(snapshot?.nodes ?? []), [snapshot])
  const needle = query.trim().toLowerCase()
  const visible = needle ? rows.filter((row) => row.label.toLowerCase().includes(needle)) : rows
  if (!snapshot) return null
  return (
    <div className="app-testing-tree">
      {snapshot.truncated && (
        <p className="app-testing-warning" role="status">
          {c.truncated}
        </p>
      )}
      {snapshot.warnings.map((warning) => (
        <p className="app-testing-warning" role="status" key={warning}>
          {warning}
        </p>
      ))}
      {visible.length === 0 ? (
        <p className="app-testing-empty">{c.noMatch}</p>
      ) : (
        <ul className="app-testing-rows" role="tree">
          {visible.map((row) => (
            <li key={row.ref} role="treeitem" aria-selected={selected === row.ref}>
              <button
                type="button"
                className={selected === row.ref ? 'active' : ''}
                style={{ paddingLeft: `${8 + row.depth * 14}px` }}
                title={row.label}
                onClick={() => onSelect(row.ref)}
              >
                {row.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

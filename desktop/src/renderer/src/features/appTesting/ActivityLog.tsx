import type { AppUiLogEntry } from '../../../../shared/appTesting'
import type { AppTestingCopy } from './copy'

const VISIBLE_ENTRIES = 15

type Props = {
  copy: AppTestingCopy
  log: AppUiLogEntry[]
}

function clock(seconds: number): string {
  return new Date(seconds * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export function ActivityLog({ copy, log }: Props) {
  const entries = log.filter((entry) => entry.action !== 'snapshot').slice(-VISIBLE_ENTRIES).reverse()
  return (
    <section className="app-testing-section">
      <header>
        <span>{copy.activity}</span>
      </header>
      {entries.length === 0 ? (
        <p className="app-testing-note">{copy.noActivity}</p>
      ) : (
        <ul className="app-testing-activity">
          {entries.map((entry, index) => (
            <li key={`${entry.time}-${index}`} className={entry.ok ? '' : 'fail'}>
              <time>{clock(entry.time)}</time>
              <b>{entry.action}</b>
              <span title={entry.error || entry.title}>{entry.error || entry.title}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

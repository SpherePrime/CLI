import { useState } from 'react'
import { AlertTriangle, CheckCircle2, ChevronDown, ChevronRight, Info, XCircle } from 'lucide-react'
import type { AppUiIssue, AppUiReport } from '../../../../shared/appTesting'
import type { AppTestingCopy } from './copy'

type Props = {
  copy: AppTestingCopy
  report?: AppUiReport
  onClear(): void
  onIssue(issue: AppUiIssue): void
}

function IssueIcon({ issue }: { issue: AppUiIssue }) {
  if (issue.severity === 'error') return <XCircle size={13} />
  if (issue.severity === 'warning') return <AlertTriangle size={13} />
  return <Info size={13} />
}

export function TestReport({ copy, report, onClear, onIssue }: Props) {
  const [layoutOpen, setLayoutOpen] = useState(true)
  const checks = report?.checks ?? []
  const audit = report?.audit
  const passed = checks.filter((check) => check.passed).length
  const failed = checks.length - passed
  return (
    <section className="app-testing-section">
      <header>
        <span>{copy.report}</span>
        {checks.length > 0 && (
          <span className="app-testing-score">
            <b className="ok">
              {passed} {copy.passed}
            </b>
            {failed > 0 && (
              <b className="fail">
                {failed} {copy.failed}
              </b>
            )}
          </span>
        )}
        {(checks.length > 0 || audit) && (
          <button type="button" className="app-testing-link" onClick={onClear}>
            {copy.clear}
          </button>
        )}
      </header>
      {checks.length === 0 ? (
        <p className="app-testing-note">{copy.noChecks}</p>
      ) : (
        <ul className="app-testing-checks">
          {checks
            .slice()
            .sort((a, b) => Number(a.passed) - Number(b.passed))
            .map((check) => (
              <li key={check.name} className={check.passed ? 'ok' : 'fail'}>
                {check.passed ? <CheckCircle2 size={14} /> : <XCircle size={14} />}
                <div>
                  <strong>{check.name}</strong>
                  {check.details && <span>{check.details}</span>}
                </div>
              </li>
            ))}
        </ul>
      )}
      {audit && (
        <div className="app-testing-audit">
          <button type="button" className="app-testing-disclosure" onClick={() => setLayoutOpen((open) => !open)}>
            {layoutOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
            {copy.layout}
            <span className="app-testing-muted">
              {audit.errors} · {audit.warnings} · {audit.checked}
            </span>
          </button>
          {layoutOpen &&
            (audit.issues.length === 0 ? (
              <p className="app-testing-note">{copy.layoutClean}</p>
            ) : (
              <ul className="app-testing-issues">
                {audit.issues.map((issue, index) => (
                  <li key={`${issue.kind}-${issue.refs.join('-')}-${index}`} className={issue.severity}>
                    <button type="button" title={copy.severity[issue.severity]} onClick={() => onIssue(issue)}>
                      <IssueIcon issue={issue} />
                      <span>{issue.message}</span>
                    </button>
                  </li>
                ))}
              </ul>
            ))}
        </div>
      )}
    </section>
  )
}

import { useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
import type { Operation } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'

export function IntegrationTools({
  workspace: w,
  kind,
  name
}: {
  workspace: Workspace
  kind: 'mcp' | 'lsp'
  name: string
}) {
  const { t } = useI18n()
  const [uri, setUri] = useState('')
  const [output, setOutput] = useState('')
  const [busy, setBusy] = useState(false)
  async function run(operation: Operation, args: Record<string, unknown> = {}) {
    setBusy(true)
    try {
      const result = await w.call(operation, args)
      setOutput(
        typeof result === 'string' ? result : JSON.stringify(result, null, 2) || t('Готово')
      )
      await w.refresh()
    } catch (error: any) {
      setOutput(error.message)
    } finally {
      setBusy(false)
    }
  }
  return (
    <details className="integration-tools">
      <summary>{t('Инструменты и диагностика')}</summary>
      <div className="button-row">
        {kind === 'mcp' ? (
          <>
            <button disabled={busy || w.busy} onClick={() => void run('mcpRefresh', { name })}>
              {t('Обновить инструменты')}
            </button>
            <button disabled={busy} onClick={() => void run('mcpPrompts')}>
              {t('Шаблоны запросов')}
            </button>
          </>
        ) : (
          <>
            <button disabled={busy} onClick={() => void run('diagnostics', { name })}>
              {t('Диагностика')}
            </button>
            <button
              disabled={busy || w.busy}
              onClick={() => void run('lspStart', { path: w.project?.path })}
            >
              {t('Запустить')}
            </button>
            <button disabled={busy || w.busy} onClick={() => void run('lspStop')}>
              {t('Остановить LSP')}
            </button>
          </>
        )}
      </div>
      {kind === 'mcp' && (
        <form
          onSubmit={(event) => {
            event.preventDefault()
            void run('mcpResource', { name, uri })
          }}
        >
          <label className="form-label">
            {t('URI ресурса')}
            <input value={uri} onChange={(event) => setUri(event.target.value)} required />
          </label>
          <button disabled={busy || !uri.trim()}>{t('Прочитать ресурс')}</button>
        </form>
      )}
      {output && (
        <pre className="skill-instructions" role="status">
          {output}
        </pre>
      )}
    </details>
  )
}

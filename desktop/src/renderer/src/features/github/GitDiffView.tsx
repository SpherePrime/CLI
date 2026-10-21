import { useMemo, useState } from 'react'
import { ChevronRight, FileCode2 } from 'lucide-react'
import { useI18n } from '../../i18n'
import { parseGitDiff } from './parseGitDiff'
import '../../styles/git-diff.css'

export function GitDiffView({ diff }: { diff: string }) {
  const { t } = useI18n()
  const files = useMemo(() => parseGitDiff(diff), [diff])
  const [expanded, setExpanded] = useState({ diff: '', count: 2000 })
  const limit = expanded.diff === diff ? expanded.count : 2000
  const totalRows = files.reduce((total, file) => total + file.rows.length, 0)
  let remaining = limit
  const visibleFiles = files.flatMap((file) => {
    const rows = file.rows.slice(0, remaining)
    remaining -= rows.length
    return rows.length || !file.rows.length ? [{ ...file, rows }] : []
  })
  if (!diff) return <p className="git-history-meta">{t('Изменений в файлах проекта нет')}</p>
  if (!files.length) return <pre className="patch-fallback">{diff}</pre>
  const added = files.reduce((total, file) => total + file.added, 0)
  const removed = files.reduce((total, file) => total + file.removed, 0)
  return (
    <div className="patch-view">
      <div className="patch-overview">
        <span>{t('Файлов: {count}', { count: files.length })}</span>
        <span className="patch-added" title={t('Добавлено строк')}>
          +{added}
        </span>
        <span className="patch-removed" title={t('Удалено строк')}>
          −{removed}
        </span>
      </div>
      {visibleFiles.map((file, fileIndex) => (
        <details className="patch-file" open key={`${file.path}:${fileIndex}`}>
          <summary>
            <ChevronRight className="patch-chevron" size={14} aria-hidden="true" />
            <FileCode2 size={15} aria-hidden="true" />
            <span className="patch-path" title={file.path}>
              {file.path}
            </span>
            <span className="patch-file-counts">
              <span className="patch-added" title={t('Добавлено строк')}>
                +{file.added}
              </span>
              <span className="patch-removed" title={t('Удалено строк')}>
                −{file.removed}
              </span>
            </span>
          </summary>
          <div className="patch-code" role="region" aria-label={file.path} tabIndex={0}>
            {file.rows.map((row, index) =>
              row.kind === 'hunk' || row.kind === 'meta' ? (
                <div className={`patch-row patch-${row.kind}`} key={index}>
                  <code>{row.text}</code>
                </div>
              ) : (
                <div className={`patch-row patch-${row.kind}`} key={index}>
                  <span className="patch-line-number" aria-hidden="true">
                    {row.oldLine}
                  </span>
                  <span className="patch-line-number" aria-hidden="true">
                    {row.newLine}
                  </span>
                  <span className="patch-marker">
                    {row.kind === 'added' ? '+' : row.kind === 'removed' ? '−' : ' '}
                  </span>
                  <code>{row.text || ' '}</code>
                </div>
              )
            )}
          </div>
        </details>
      ))}
      {totalRows > limit && (
        <div className="patch-more">
          <span>{t('Показано строк: {shown} из {total}', { shown: limit, total: totalRows })}</span>
          <button onClick={() => setExpanded({ diff, count: limit + 2000 })}>
            {t('Показать ещё')}
          </button>
        </div>
      )}
    </div>
  )
}

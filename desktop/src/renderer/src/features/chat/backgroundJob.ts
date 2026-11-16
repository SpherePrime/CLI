export function backgroundJob(metadata: unknown) {
  let value: unknown = metadata
  if (typeof metadata === 'string') {
    if (!metadata) return ''
    try {
      value = JSON.parse(metadata)
    } catch {
      return ''
    }
  }
  if (!value || typeof value !== 'object') return ''
  const record = value as { background?: unknown; job_id?: unknown; shell_id?: unknown }
  if (record.background !== true) return ''
  if (typeof record.job_id === 'string' && record.job_id) return record.job_id
  if (typeof record.shell_id === 'string' && record.shell_id) return record.shell_id
  return ''
}

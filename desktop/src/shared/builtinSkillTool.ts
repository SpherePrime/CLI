const skillReadToolNames = new Set(['view', 'read', 'read_file'])

function isBuiltinSkillPath(rawPath: unknown): boolean {
  if (typeof rawPath !== 'string') return false
  const normalized = rawPath.trim().replace(/\\/g, '/').toLowerCase()
  if (!normalized) return false
  if (normalized.startsWith('prime://skills/')) return true
  return /(^|\/)builtin-skills\/[^/]+\//.test(normalized)
}

function extractInputPath(input: unknown): string | null {
  if (input && typeof input === 'object') {
    const record = input as Record<string, unknown>
    const value = record.file_path ?? record.path ?? record.filePath
    return typeof value === 'string' ? value : null
  }
  if (typeof input !== 'string') return null
  const trimmed = input.trim()
  if (!trimmed) return null
  try {
    const parsed = JSON.parse(trimmed)
    if (parsed && typeof parsed === 'object') {
      const record = parsed as Record<string, unknown>
      const value = record.file_path ?? record.path ?? record.filePath
      if (typeof value === 'string') return value
    }
  } catch {
    const partial = /"(?:file_path|path|filePath)"\s*:\s*"([^"]*)/i.exec(trimmed)
    if (partial) return partial[1].replace(/\\\\/g, '\\')
  }
  return null
}

export function isBuiltinSkillReadTool(name: unknown, input: unknown): boolean {
  const normalizedName = String(name || '').trim().toLowerCase()
  if (!skillReadToolNames.has(normalizedName)) return false
  const targetPath = extractInputPath(input)
  return isBuiltinSkillPath(targetPath)
}

const internalDocumentationTools = new Set(['read_tool_documentation', 'read_instruction'])

const skillFileTools = new Set(['view', 'read', 'read_file'])

function skillPath(path: unknown) {
  if (typeof path !== 'string' || !path) return false
  const normalized = path.replace(/\\/g, '/')
  return (
    normalized.startsWith('prime://') ||
    /(^|\/)SKILL\.md$/i.test(normalized) ||
    /(^|\/)skills\//i.test(normalized)
  )
}

export function hidesToolOutput(name: string, input: unknown) {
  if (internalDocumentationTools.has(name)) return true
  if (!skillFileTools.has(name) || !input || typeof input !== 'object') return false
  const values = input as Record<string, unknown>
  return skillPath(values.file_path ?? values.path)
}

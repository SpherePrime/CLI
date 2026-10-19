export type ContextSource = {
  id: string
  kind: 'file' | 'selection'
  path: string
  content?: string
  startLine?: number
  endLine?: number
}
export type AgentContext = {
  enabled: boolean
  goal: string
  constraints: string
  criteria: string
  sources: ContextSource[]
}

export function emptyAgentContext(): AgentContext {
  return { enabled: false, goal: '', constraints: '', criteria: '', sources: [] }
}

export function mergeContextSource(context: AgentContext, source: ContextSource): AgentContext {
  if (source.kind === 'selection' && (source.content?.length || 0) > 12000)
    throw new Error('Selection exceeds 12000 characters')
  const remaining = context.sources.filter((item) => item.id !== source.id)
  if (remaining.length >= 8) throw new Error('Maximum 8 context sources')
  return { ...context, enabled: true, sources: [...remaining, { ...source }] }
}

export async function buildAgentPrompt(
  prompt: string,
  context: AgentContext,
  read: (path: string) => Promise<{ content?: string; image?: string }>
): Promise<string> {
  if (!context.enabled || prompt.startsWith('!')) return prompt
  const sections: string[] = []
  if (context.goal.trim()) sections.push(`Task goal:\n${context.goal.trim().slice(0, 4000)}`)
  if (context.constraints.trim())
    sections.push(`Constraints:\n${context.constraints.trim().slice(0, 4000)}`)
  if (context.criteria.trim())
    sections.push(`Acceptance criteria:\n${context.criteria.trim().slice(0, 4000)}`)
  for (const source of context.sources.slice(0, 8)) {
    let content = source.content || ''
    if (source.kind === 'file') {
      try {
        const file = await read(source.path)
        if (file.image || typeof file.content !== 'string') throw new Error('Text file required')
        content = file.content
      } catch (error) {
        throw new Error(`${source.path}: ${error instanceof Error ? error.message : error}`)
      }
    }
    const location =
      source.kind === 'selection'
        ? `${source.path}:${source.startLine}-${source.endLine} (editor selection snapshot)`
        : `${source.path} (current file on disk)`
    sections.push(
      `Source: ${location}\n<source-content>\n${content.slice(0, 12000)}\n</source-content>${content.length > 12000 ? '\n[Excerpt truncated; read the file for the rest.]' : ''}`
    )
  }
  if (!sections.length) return prompt
  return `${prompt}\n\n<working-context>\n${sections.join('\n\n')}\n</working-context>`
}

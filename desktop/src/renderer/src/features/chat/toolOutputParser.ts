export type ToolCategory =
  | 'edit'
  | 'write'
  | 'read'
  | 'command'
  | 'search'
  | 'browser'
  | 'task'
  | 'other'

export interface ParsedToolOutput {
  cleanText: string
  cwd: string | null
  diagnostics: string | null
  diagnosticSummary: string | null
  exitCode: number | null
}

export function classifyToolCategory(name: string, isShell = false): ToolCategory {
  if (isShell) return 'command'
  const normalized = String(name || '').toLowerCase()
  if (
    normalized === 'edit' ||
    normalized === 'multiedit' ||
    normalized === 'lsp_rename' ||
    normalized === 'lsp_replace_symbol' ||
    normalized.includes('ssh_edit_file')
  ) {
    return 'edit'
  }
  if (
    normalized === 'write' ||
    normalized === 'download' ||
    normalized === 'generate_image' ||
    normalized === 'pdf_document' ||
    normalized === 'word_document' ||
    normalized === 'excel_workbook' ||
    normalized === 'powerpoint_presentation' ||
    normalized.includes('ssh_write_file')
  ) {
    return 'write'
  }
  if (
    normalized === 'view' ||
    normalized === 'read' ||
    normalized === 'read_file' ||
    normalized === 'read_tool_result' ||
    normalized === 'lsp_symbols' ||
    normalized === 'lsp_definition' ||
    normalized === 'lsp_references' ||
    normalized === 'lsp_diagnostics' ||
    normalized === 'lsp_call_hierarchy' ||
    normalized === 'lsp_restart' ||
    normalized.includes('ssh_read_file')
  ) {
    return 'read'
  }
  if (
    normalized === 'bash' ||
    normalized === 'sudo' ||
    normalized === 'job_output' ||
    normalized === 'job_kill' ||
    normalized.includes('terminal_') ||
    normalized.includes('ssh_exec')
  ) {
    return 'command'
  }
  if (
    normalized === 'grep' ||
    normalized === 'glob' ||
    normalized === 'ls' ||
    normalized === 'web_search' ||
    normalized === 'web_fetch' ||
    normalized === 'sourcegraph' ||
    normalized === 'fetch' ||
    normalized === 'agentic_fetch'
  ) {
    return 'search'
  }
  if (normalized.includes('browser_') || normalized.includes('window_') || normalized.includes('app_')) {
    return 'browser'
  }
  if (
    normalized === 'agent' ||
    normalized === 'agent_jobs' ||
    normalized === 'agent_message' ||
    normalized.startsWith('custom_agent_')
  ) {
    return 'task'
  }
  return 'other'
}

function peelTrailingBlock(text: string, tag: string): { content: string; rest: string } | null {
  const trimmed = text.trimEnd()
  const closeTag = `</${tag}>`
  if (!trimmed.endsWith(closeTag)) return null
  const openTag = `<${tag}>`
  const openIndex = trimmed.lastIndexOf(openTag)
  if (openIndex < 0) return null
  const lineStart = trimmed.lastIndexOf('\n', openIndex - 1) + 1
  if (trimmed.slice(lineStart, openIndex).trim() !== '') return null
  const content = trimmed.slice(openIndex + openTag.length, trimmed.length - closeTag.length)
  return { content, rest: trimmed.slice(0, lineStart) }
}

export function parseToolOutput(raw: string): ParsedToolOutput {
  if (!raw) {
    return { cleanText: '', cwd: null, diagnostics: null, diagnosticSummary: null, exitCode: null }
  }
  let text = String(raw).replace(/\r\n/g, '\n')
  let cwd: string | null = null
  let diagnostics: string | null = null
  let diagnosticSummary: string | null = null
  let exitCode: number | null = null

  while (true) {
    if (diagnostics === null) {
      const diag = peelTrailingBlock(text, 'project_diagnostics')
      if (diag) {
        diagnostics = diag.content.trim() || null
        text = diag.rest
        continue
      }
    }
    if (diagnosticSummary === null) {
      const summary = peelTrailingBlock(text, 'diagnostic_summary')
      if (summary) {
        diagnosticSummary = summary.content
          .trim()
          .split('\n')
          .map((line) => line.trim())
          .filter(Boolean)
          .join(' · ')
        text = summary.rest
        continue
      }
    }
    if (cwd === null) {
      const cwdBlock = peelTrailingBlock(text, 'cwd')
      if (cwdBlock) {
        cwd = cwdBlock.content.trim() || null
        text = cwdBlock.rest
        continue
      }
    }
    break
  }

  text = text
    .replace(/^\s*<result>\s*/i, '')
    .replace(/\s*<\/result>\s*$/i, '')
    .replace(/^\s*<file>\s*/i, '')
    .replace(/\s*<\/file>\s*$/i, '')

  const exitMatch = text.match(/(?:^|\n)Exit code\s+(\d+)\s*$/i)
  if (exitMatch) {
    exitCode = Number(exitMatch[1])
  }

  return {
    cleanText: text.trim(),
    cwd,
    diagnostics,
    diagnosticSummary,
    exitCode
  }
}

export function splitFilePath(rawPath: string): { dir: string; base: string } {
  const normalized = String(rawPath || '').replace(/\\/g, '/')
  const idx = normalized.lastIndexOf('/')
  if (idx < 0) return { dir: '', base: normalized }
  return {
    dir: normalized.slice(0, idx + 1),
    base: normalized.slice(idx + 1)
  }
}

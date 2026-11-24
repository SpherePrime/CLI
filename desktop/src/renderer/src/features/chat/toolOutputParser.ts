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
  if (normalized === 'edit' || normalized === 'multiedit' || normalized === 'lsp_rename' || normalized === 'lsp_replace_symbol') {
    return 'edit'
  }
  if (normalized === 'write' || normalized === 'download' || normalized === 'generate_image') {
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
    normalized === 'lsp_diagnostics'
  ) {
    return 'read'
  }
  if (
    normalized === 'bash' ||
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
    normalized === 'sourcegraph' ||
    normalized === 'fetch' ||
    normalized === 'agentic_fetch'
  ) {
    return 'search'
  }
  if (normalized.includes('browser_') || normalized.includes('window_') || normalized.includes('app_')) {
    return 'browser'
  }
  if (normalized === 'agent' || normalized === 'agent_jobs' || normalized === 'agent_message') {
    return 'task'
  }
  return 'other'
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

  const diagMatch = text.match(/<project_diagnostics>([\s\S]*?)<\/project_diagnostics>/)
  if (diagMatch) {
    diagnostics = diagMatch[1].trim() || null
    text = text.replace(diagMatch[0], '')
  }

  const summaryMatch = text.match(/<diagnostic_summary>([\s\S]*?)<\/diagnostic_summary>/)
  if (summaryMatch) {
    diagnosticSummary = summaryMatch[1]
      .trim()
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)
      .join(' · ')
    text = text.replace(summaryMatch[0], '')
  }

  const cwdMatch = text.match(/<cwd>([\s\S]*?)<\/cwd>/)
  if (cwdMatch) {
    cwd = cwdMatch[1].trim() || null
    text = text.replace(cwdMatch[0], '')
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

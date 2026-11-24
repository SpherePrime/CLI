export type DiffLineKind = 'context' | 'add' | 'del'

export interface DiffLine {
  kind: DiffLineKind
  oldLine: number | null
  newLine: number | null
  text: string
}

export interface DiffBlock {
  lines: DiffLine[]
  added: number
  removed: number
  truncated: boolean
}

export interface DiffStats {
  added: number
  removed: number
}

export interface CodeToken {
  kind: 'plain' | 'kw' | 'str' | 'fn' | 'num'
  text: string
}

const MAX_LCS_CELLS = 160_000
const MAX_RENDER_LINES = 240
const CONTEXT_RADIUS = 2

const KEYWORDS = new Set([
  'async',
  'await',
  'break',
  'case',
  'catch',
  'class',
  'const',
  'continue',
  'default',
  'defer',
  'def',
  'elif',
  'else',
  'enum',
  'export',
  'extends',
  'false',
  'finally',
  'fn',
  'for',
  'from',
  'func',
  'function',
  'go',
  'if',
  'impl',
  'import',
  'in',
  'interface',
  'let',
  'match',
  'mod',
  'mut',
  'new',
  'nil',
  'null',
  'package',
  'private',
  'pub',
  'public',
  'return',
  'self',
  'struct',
  'switch',
  'throw',
  'trait',
  'true',
  'try',
  'type',
  'undefined',
  'use',
  'var',
  'void',
  'where',
  'while'
])

function splitLines(text: string): string[] {
  if (!text) return []
  return text.replace(/\r\n/g, '\n').split('\n')
}

function buildLcsDiff(oldLines: string[], newLines: string[]): DiffLine[] {
  const m = oldLines.length
  const n = newLines.length
  if (m * n > MAX_LCS_CELLS) {
    const fallback: DiffLine[] = []
    oldLines.forEach((text, index) => {
      fallback.push({ kind: 'del', oldLine: index + 1, newLine: null, text })
    })
    newLines.forEach((text, index) => {
      fallback.push({ kind: 'add', oldLine: null, newLine: index + 1, text })
    })
    return fallback
  }

  const dp = new Uint32Array((m + 1) * (n + 1))
  const stride = n + 1
  for (let i = m - 1; i >= 0; i -= 1) {
    const row = i * stride
    const nextRow = (i + 1) * stride
    for (let j = n - 1; j >= 0; j -= 1) {
      if (oldLines[i] === newLines[j]) {
        dp[row + j] = dp[nextRow + j + 1] + 1
      } else {
        const down = dp[nextRow + j]
        const right = dp[row + j + 1]
        dp[row + j] = down >= right ? down : right
      }
    }
  }

  const result: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < m && j < n) {
    if (oldLines[i] === newLines[j]) {
      result.push({ kind: 'context', oldLine: i + 1, newLine: j + 1, text: oldLines[i] })
      i += 1
      j += 1
    } else if (dp[(i + 1) * stride + j] >= dp[i * stride + j + 1]) {
      result.push({ kind: 'del', oldLine: i + 1, newLine: null, text: oldLines[i] })
      i += 1
    } else {
      result.push({ kind: 'add', oldLine: null, newLine: j + 1, text: newLines[j] })
      j += 1
    }
  }
  while (i < m) {
    result.push({ kind: 'del', oldLine: i + 1, newLine: null, text: oldLines[i] })
    i += 1
  }
  while (j < n) {
    result.push({ kind: 'add', oldLine: null, newLine: j + 1, text: newLines[j] })
    j += 1
  }
  return result
}

function trimContext(lines: DiffLine[]): DiffLine[] {
  const changedIndexes: number[] = []
  lines.forEach((line, index) => {
    if (line.kind !== 'context') changedIndexes.push(index)
  })
  if (changedIndexes.length === 0) return lines.slice(0, 12)

  const keep = new Uint8Array(lines.length)
  for (const idx of changedIndexes) {
    const start = Math.max(0, idx - CONTEXT_RADIUS)
    const end = Math.min(lines.length - 1, idx + CONTEXT_RADIUS)
    for (let k = start; k <= end; k += 1) {
      keep[k] = 1
    }
  }
  return lines.filter((_, index) => keep[index] === 1)
}

export function computeEditDiff(oldText: string, newText: string): DiffBlock {
  const raw = buildLcsDiff(splitLines(oldText), splitLines(newText))
  let added = 0
  let removed = 0
  for (const line of raw) {
    if (line.kind === 'add') added += 1
    else if (line.kind === 'del') removed += 1
  }
  const focused = trimContext(raw)
  const truncated = focused.length > MAX_RENDER_LINES
  return {
    lines: truncated ? focused.slice(0, MAX_RENDER_LINES) : focused,
    added,
    removed,
    truncated
  }
}

export function extractToolDiff(name: string, input: any): DiffBlock | null {
  if (!input || typeof input !== 'object') return null
  if (name === 'edit') {
    const oldText = typeof input.old_string === 'string' ? input.old_string : ''
    const newText = typeof input.new_string === 'string' ? input.new_string : ''
    if (!oldText && !newText) return null
    return computeEditDiff(oldText, newText)
  }
  if (name === 'multiedit' && Array.isArray(input.edits) && input.edits.length > 0) {
    const merged: DiffLine[] = []
    let added = 0
    let removed = 0
    let truncated = false
    for (const entry of input.edits) {
      if (!entry || typeof entry !== 'object') continue
      const block = computeEditDiff(
        typeof entry.old_string === 'string' ? entry.old_string : '',
        typeof entry.new_string === 'string' ? entry.new_string : ''
      )
      added += block.added
      removed += block.removed
      if (block.truncated) truncated = true
      for (const line of block.lines) {
        if (merged.length >= MAX_RENDER_LINES) {
          truncated = true
          break
        }
        merged.push(line)
      }
    }
    return { lines: merged, added, removed, truncated }
  }
  return null
}

export function extractToolDiffStats(name: string, input: any): DiffStats | null {
  if (!input || typeof input !== 'object') return null
  if (name === 'edit' || name === 'multiedit') {
    const diff = extractToolDiff(name, input)
    if (!diff || (diff.added === 0 && diff.removed === 0)) return null
    return { added: diff.added, removed: diff.removed }
  }
  if (name === 'write' && typeof input.content === 'string') {
    const added = splitLines(input.content).length
    if (added === 0) return null
    return { added, removed: 0 }
  }
  return null
}

export function buildDiffMeter(stats: DiffStats): Array<'add' | 'del' | 'idle'> {
  const total = stats.added + stats.removed
  if (total <= 0) return ['idle', 'idle', 'idle', 'idle', 'idle']
  const slots = 5
  let addSlots = Math.round((stats.added / total) * slots)
  let delSlots = Math.round((stats.removed / total) * slots)
  if (stats.added > 0 && addSlots === 0) addSlots = 1
  if (stats.removed > 0 && delSlots === 0) delSlots = 1
  while (addSlots + delSlots > slots) {
    if (addSlots >= delSlots && addSlots > 1) addSlots -= 1
    else if (delSlots > 1) delSlots -= 1
    else break
  }
  const idleSlots = Math.max(0, slots - addSlots - delSlots)
  return [
    ...Array<'add'>(addSlots).fill('add'),
    ...Array<'del'>(delSlots).fill('del'),
    ...Array<'idle'>(idleSlots).fill('idle')
  ]
}

const TOKEN_REGEX =
  /('(?:\\.|[^'\\])*'|"(?:\\.|[^"\\])*"|`(?:\\.|[^`\\])*`|\b\d+(?:\.\d+)?\b|\b[A-Za-z_][A-Za-z0-9_]*\b(?=\s*\()|\b[A-Za-z_][A-Za-z0-9_]*\b)/g

export function tokenizeCodeLine(line: string): CodeToken[] {
  if (!line) return []
  const tokens: CodeToken[] = []
  let cursor = 0
  TOKEN_REGEX.lastIndex = 0
  let match: RegExpExecArray | null
  while ((match = TOKEN_REGEX.exec(line)) !== null) {
    if (match.index > cursor) {
      tokens.push({ kind: 'plain', text: line.slice(cursor, match.index) })
    }
    const text = match[0]
    const first = text[0]
    let kind: CodeToken['kind'] = 'plain'
    if (first === "'" || first === '"' || first === '`') {
      kind = 'str'
    } else if (first >= '0' && first <= '9') {
      kind = 'num'
    } else if (KEYWORDS.has(text)) {
      kind = 'kw'
    } else if (line[match.index + text.length] === '(') {
      kind = 'fn'
    }
    tokens.push({ kind, text })
    cursor = match.index + text.length
  }
  if (cursor < line.length) {
    tokens.push({ kind: 'plain', text: line.slice(cursor) })
  }
  return tokens
}

export type DiffRow = {
  kind: 'added' | 'removed' | 'context' | 'hunk' | 'meta'
  text: string
  oldLine?: number
  newLine?: number
}

export type DiffFile = { path: string; added: number; removed: number; rows: DiffRow[] }

function filePath(value: string, stripPrefix = true) {
  let path = value.split('\t')[0]
  if (path.startsWith('"')) {
    try {
      path = JSON.parse(path)
    } catch {
      path = path.slice(1, -1)
    }
  }
  return stripPrefix ? path.replace(/^[ab]\//, '') : path
}

export function parseGitDiff(diff: string): DiffFile[] {
  const files: DiffFile[] = []
  let file: DiffFile | undefined
  let oldLine = 0
  let newLine = 0
  let inHunk = false
  for (const line of diff.replace(/\r\n/g, '\n').split('\n')) {
    if (line.startsWith('diff --git ')) {
      const header = line.match(
        /^diff --git ("a\/(?:\\.|[^"])*"|a\/.*?) ("b\/(?:\\.|[^"])*"|b\/.*)$/
      )
      file = { path: filePath(header?.[2] || line.slice(11)), added: 0, removed: 0, rows: [] }
      files.push(file)
      inHunk = false
      continue
    }
    if (!file) continue
    if (!inHunk && line.startsWith('--- ')) {
      if (line.slice(4) !== '/dev/null') file.path = filePath(line.slice(4))
      continue
    }
    if (!inHunk && line.startsWith('+++ ')) {
      if (line.slice(4) !== '/dev/null') file.path = filePath(line.slice(4))
      continue
    }
    if (!inHunk && line.startsWith('rename to ')) file.path = filePath(line.slice(10), false)
    const hunk = line.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/)
    if (hunk) {
      oldLine = Number(hunk[1])
      newLine = Number(hunk[2])
      inHunk = true
      file.rows.push({ kind: 'hunk', text: line })
    } else if (inHunk && line.startsWith('+')) {
      file.added++
      file.rows.push({ kind: 'added', text: line.slice(1), newLine: newLine++ })
    } else if (inHunk && line.startsWith('-')) {
      file.removed++
      file.rows.push({ kind: 'removed', text: line.slice(1), oldLine: oldLine++ })
    } else if (inHunk && line.startsWith(' ')) {
      file.rows.push({
        kind: 'context',
        text: line.slice(1),
        oldLine: oldLine++,
        newLine: newLine++
      })
    } else if (line && !line.startsWith('index ')) {
      file.rows.push({ kind: 'meta', text: line })
    }
  }
  return files
}

import { useEffect, useLayoutEffect, useRef } from 'react'
import { monaco } from './monaco'
export function modelUri(workspaceId: string, path: string) {
  return monaco.Uri.parse(`prime://${workspaceId}/${encodeURI(path.replaceAll('\\', '/'))}`)
}
export function CodeEditor({
  workspaceId,
  path,
  value,
  original,
  language,
  theme,
  onChange,
  onPosition,
  label,
  onSelection
}: {
  workspaceId: string
  path: string
  value: string
  original?: string
  language: string
  theme: string
  onChange: (value: string) => void
  onPosition: (position: { line: number; column: number }) => void
  label: string
  onSelection?: (selection: { content: string; startLine: number; endLine: number } | null) => void
}) {
  const container = useRef<HTMLDivElement>(null)
  const instance = useRef<
    monaco.editor.IStandaloneCodeEditor | monaco.editor.IStandaloneDiffEditor | null
  >(null)
  const changes = useRef(onChange)
  const positions = useRef(onPosition)
  const selections = useRef(onSelection)
  changes.current = onChange
  positions.current = onPosition
  selections.current = onSelection
  useLayoutEffect(() => {
    const options = {
      theme,
      fontFamily: 'JetBrains Mono, monospace',
      fontSize: 13,
      lineHeight: 21,
      minimap: { enabled: false },
      automaticLayout: true,
      scrollBeyondLastLine: false,
      padding: { top: 16 },
      tabSize: 2,
      ariaLabel: label,
      bracketPairColorization: { enabled: true }
    }
    if (original !== undefined) {
      const before = monaco.editor.createModel(original, language)
      const after = monaco.editor.createModel(value, language)
      const editor = monaco.editor.createDiffEditor(container.current!, {
        ...options,
        readOnly: true,
        renderSideBySide: true
      })
      editor.setModel({ original: before, modified: after })
      instance.current = editor
      return () => {
        instance.current = null
        editor.setModel(null)
        editor.dispose()
        before.dispose()
        after.dispose()
      }
    }
    const uri = modelUri(workspaceId, path)
    const model = monaco.editor.getModel(uri) || monaco.editor.createModel(value, language, uri)
    const editor = monaco.editor.create(container.current!, { ...options, model })
    instance.current = editor
    const change = editor.onDidChangeModelContent(() => changes.current(editor.getValue()))
    const position = editor.onDidChangeCursorPosition((event) =>
      positions.current({ line: event.position.lineNumber, column: event.position.column })
    )
    const selection = editor.onDidChangeCursorSelection((event) =>
      selections.current?.(
        event.selection.isEmpty()
          ? null
          : {
              content: model.getValueInRange(event.selection),
              startLine: event.selection.startLineNumber,
              endLine: event.selection.endLineNumber
            }
      )
    )
    editor.focus()
    return () => {
      instance.current = null
      change.dispose()
      position.dispose()
      selection.dispose()
      editor.setModel(null)
      editor.dispose()
    }
  }, [workspaceId, path, original !== undefined])
  useEffect(() => {
    monaco.editor.setTheme(theme)
  }, [theme])
  useEffect(() => {
    if (original !== undefined) return
    const editor = instance.current as monaco.editor.IStandaloneCodeEditor | null
    if (editor && editor.getValue() !== value) editor.setValue(value)
  }, [value, original, path, workspaceId])
  return <div className="code-editor-host" ref={container} />
}

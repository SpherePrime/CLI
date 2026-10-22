import * as monaco from 'monaco-editor'
import EditorWorker from 'monaco-editor/editor/editor.worker.js?worker'
import JsonWorker from 'monaco-editor/language/json/json.worker.js?worker'
import CssWorker from 'monaco-editor/language/css/css.worker.js?worker'
import HtmlWorker from 'monaco-editor/language/html/html.worker.js?worker'
import TsWorker from 'monaco-editor/language/typescript/ts.worker.js?worker'
self.MonacoEnvironment = {
  getWorker: (_moduleId, label) =>
    label === 'json'
      ? new JsonWorker()
      : ['css', 'scss', 'less'].includes(label)
        ? new CssWorker()
        : ['html', 'handlebars', 'razor'].includes(label)
          ? new HtmlWorker()
          : ['typescript', 'javascript'].includes(label)
            ? new TsWorker()
            : new EditorWorker()
}
export { monaco }
export function languageFor(path: string) {
  const extension = path.split('.').pop()?.toLowerCase()
  return (
    (
      {
        ts: 'typescript',
        tsx: 'typescript',
        js: 'javascript',
        jsx: 'javascript',
        mjs: 'javascript',
        cjs: 'javascript',
        json: 'json',
        go: 'go',
        py: 'python',
        rs: 'rust',
        css: 'css',
        scss: 'scss',
        html: 'html',
        md: 'markdown',
        yml: 'yaml',
        yaml: 'yaml',
        sh: 'shell',
        ps1: 'powershell',
        sql: 'sql',
        xml: 'xml',
        toml: 'ini',
        cs: 'csharp',
        cpp: 'cpp',
        c: 'c',
        h: 'cpp',
        java: 'java',
        kt: 'kotlin',
        vue: 'html',
        txt: 'plaintext'
      } as Record<string, string>
    )[extension || ''] || 'plaintext'
  )
}

monaco.editor.defineTheme('prime-dark', {
  base: 'vs-dark',
  inherit: true,
  rules: [],
  colors: {
    'editor.background': '#101010',
    'editor.foreground': '#F1EFE9',
    'editorLineNumber.foreground': '#615e58',
    'editorLineNumber.activeForeground': '#FF8C60',
    'editor.selectionBackground': '#513425',
    'editorCursor.foreground': '#FF8C60',
    'editor.lineHighlightBackground': '#191919',
    'editorWidget.background': '#191919',
    'editorIndentGuide.background1': '#2b2b28'
  }
})
monaco.editor.defineTheme('prime-light', {
  base: 'vs',
  inherit: true,
  rules: [],
  colors: {
    'editor.background': '#faf9f5',
    'editor.foreground': '#17110d',
    'editorCursor.foreground': '#b84619',
    'editor.selectionBackground': '#f8dfd1'
  }
})

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
    'editor.background': '#151618',
    'editor.foreground': '#F5F3EE',
    'editorLineNumber.foreground': '#969792',
    'editorLineNumber.activeForeground': '#FFAD85',
    'editor.selectionBackground': '#50382D',
    'editorCursor.foreground': '#FFAD85',
    'editor.lineHighlightBackground': '#222327',
    'editorWidget.background': '#222327',
    'editorIndentGuide.background1': '#383A40'
  }
})
monaco.editor.defineTheme('prime-light', {
  base: 'vs',
  inherit: true,
  rules: [],
  colors: {
    'editor.background': '#FAF9F6',
    'editor.foreground': '#252622',
    'editorCursor.foreground': '#AE4820',
    'editor.selectionBackground': '#F8E8DD',
    'editorLineNumber.foreground': '#72736C',
    'editorLineNumber.activeForeground': '#AE4820',
    'editor.lineHighlightBackground': '#EFEEE9',
    'editorWidget.background': '#FFFFFF',
    'editorIndentGuide.background1': '#D9D7CF'
  }
})

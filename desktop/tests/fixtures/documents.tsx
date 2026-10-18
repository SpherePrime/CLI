import React, { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import FileEditor from '../../src/renderer/src/features/files/FileEditor'
import { useDocuments } from '../../src/renderer/src/features/files/useDocuments'
import '../../src/renderer/src/styles/global.css'
import '../../src/renderer/src/styles/brand.css'
let finish: (() => void) | undefined
window.prime = {
  editorDirty: async () => {},
  readFile: async (_id: string, path: string) => ({
    name: path,
    path,
    content: 'original',
    revision: 'revision'
  }),
  saveFile: async () => {
    await new Promise<void>((resolve) => {
      finish = resolve
    })
    return { revision: 'next' }
  }
} as any
window.addEventListener('fixture:complete-save', () => finish?.())
function DocumentsFixture() {
  const [error, setError] = useState('')
  const documents = useDocuments((error) => setError(String(error)))
  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <div>
        <button onClick={() => void documents.open('workspace', 'A.ts')}>Open A</button>
        <button onClick={() => void documents.open('workspace', 'B.ts')}>Open B</button>
      </div>
      {error && <p role="alert">{error}</p>}
      {documents.documents.length > 0 && (
        <div style={{ flex: 1, minHeight: 0, display: 'flex' }}>
          <FileEditor state={documents} theme="dark" />
        </div>
      )}
    </div>
  )
}
createRoot(document.getElementById('root')!).render(<DocumentsFixture />)

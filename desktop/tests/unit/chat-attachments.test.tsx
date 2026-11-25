import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { WorkspaceMediaProvider } from '../../src/renderer/src/features/chat/WorkspaceMediaContext'
import { MessageAttachment } from '../../src/renderer/src/features/chat/MessageAttachment'
import { clipboardFiles, fileUploads } from '../../src/renderer/src/features/chat/pasteImages'

function makeTransfer(files: File[], items: Array<{ kind: string; file?: File }> = []): DataTransfer {
  return {
    files,
    items: items.map((entry) => ({
      kind: entry.kind,
      getAsFile: () => entry.file ?? null
    }))
  } as unknown as DataTransfer
}

describe('clipboardFiles and fileUploads', () => {
  it('extracts documents and images while ignoring text-only paste', async () => {
    const docx = new File([new Uint8Array([0x50, 0x4b, 0x03, 0x04])], 'Отчёт.docx', {
      type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
    })
    const pdf = new File([new Uint8Array([0x25, 0x50, 0x44, 0x46])], 'contract.pdf', {
      type: 'application/pdf'
    })
    const png = new File([new Uint8Array([137, 80, 78, 71])], 'screen.png', {
      type: 'image/png'
    })

    const mixed = makeTransfer([docx], [
      { kind: 'file', file: docx },
      { kind: 'file', file: pdf },
      { kind: 'file', file: png },
      { kind: 'string' }
    ])
    const extracted = clipboardFiles(mixed)
    expect(extracted.map((file) => file.name)).toEqual(['Отчёт.docx', 'contract.pdf', 'screen.png'])

    const uploads = await fileUploads(extracted)
    expect(uploads).toHaveLength(3)
    expect(uploads[0].name).toBe('Отчёт.docx')
    expect(uploads[0].data).toBeInstanceOf(Uint8Array)
    expect(uploads[1].name).toBe('contract.pdf')

    const textOnly = makeTransfer([], [{ kind: 'string' }])
    expect(clipboardFiles(textOnly)).toEqual([])
  })
})

describe('MessageAttachment document rendering', () => {
  it('renders clean document badge/card for docx, xlsx, pptx, and pdf with open/reveal actions', () => {
    const cases = [
      {
        path: 'C:\\data\\session-attachments\\s1\\Отчёт.docx',
        mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        expectedName: 'Отчёт.docx',
        expectedExt: 'DOCX'
      },
      {
        path: '/data/session-attachments/s1/budget.xlsx',
        mime_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        expectedName: 'budget.xlsx',
        expectedExt: 'XLSX'
      },
      {
        path: '/data/session-attachments/s1/deck.pptx',
        mime_type: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
        expectedName: 'deck.pptx',
        expectedExt: 'PPTX'
      },
      {
        path: '/data/session-attachments/s1/contract.pdf',
        mime_type: 'application/pdf',
        expectedName: 'contract.pdf',
        expectedExt: 'PDF'
      }
    ]

    for (const item of cases) {
      const markup = renderToStaticMarkup(
        <I18nProvider language="ru">
          <WorkspaceMediaProvider workspaceId="ws-1">
            <MessageAttachment data={{ path: item.path, mime_type: item.mime_type }} />
          </WorkspaceMediaProvider>
        </I18nProvider>
      )
      expect(markup).toContain('attachment-document-card')
      expect(markup).toContain(item.expectedName)
      expect(markup).toContain(item.expectedExt)
      expect(markup).toContain('Открыть')
      expect(markup).toContain('Показать в папке')
      expect(markup).not.toContain('Вложение ·')
    }
  })
})

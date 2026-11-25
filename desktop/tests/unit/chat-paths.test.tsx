import { isValidElement, type ReactElement, type ReactNode } from 'react'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider, translate } from '../../src/renderer/src/i18n'
import { WorkspaceMediaProvider } from '../../src/renderer/src/features/chat/WorkspaceMediaContext'
import { MarkdownText } from '../../src/renderer/src/features/chat/MarkdownText'
import {
  clearChatPathInspectionCache,
  seedChatPathInspectionCache
} from '../../src/renderer/src/features/chat/ChatPathEntry'
import { ChatPathFileView } from '../../src/renderer/src/features/chat/ChatPathFileView'
import {
  extractTextPathSegments,
  isDocumentPath,
  isExplicitRelativePathCandidate,
  isLocalPathCandidate
} from '../../src/renderer/src/features/chat/chatPaths'

function findButtons(node: ReactNode, out: ReactElement<Record<string, any>>[] = []) {
  if (!node) return out
  if (Array.isArray(node)) {
    for (const item of node) findButtons(item, out)
    return out
  }
  if (isValidElement<Record<string, any>>(node)) {
    if (node.type === 'button') out.push(node)
    if (node.props?.children) findButtons(node.props.children as ReactNode, out)
  }
  return out
}

afterEach(() => {
  clearChatPathInspectionCache()
  delete (globalThis as Record<string, unknown>).window
})

describe('chatPaths detection', () => {
  it('extracts Windows, POSIX and relative paths while keeping surrounding punctuation and URLs intact', () => {
    const line = 'Смотри C:\\Users\\dev\\frames\\sheet_001.png, /tmp/prime-gamedev-1/run.webm и https://example.com/a.png!'
    const segments = extractTextPathSegments(line)
    expect(segments).toEqual([
      { type: 'text', value: 'Смотри ' },
      { type: 'path', value: 'C:\\Users\\dev\\frames\\sheet_001.png' },
      { type: 'text', value: ', ' },
      { type: 'path', value: '/tmp/prime-gamedev-1/run.webm' },
      { type: 'text', value: ' и https://example.com/a.png!' }
    ])
  })

  it('extracts Windows and POSIX document paths containing spaces in folder or file names', () => {
    const line =
      'Путь к файлу: C:\\Users\\dwert\\OneDrive\\Documents\\Prime\\Новый проект 4\\Prime_Desktop_Capabilities.pdf!'
    expect(extractTextPathSegments(line)).toEqual([
      { type: 'text', value: 'Путь к файлу: ' },
      {
        type: 'path',
        value: 'C:\\Users\\dwert\\OneDrive\\Documents\\Prime\\Новый проект 4\\Prime_Desktop_Capabilities.pdf'
      },
      { type: 'text', value: '!' }
    ])
  })

  it('rejects HTTP URLs, prime links, and version fractions', () => {
    expect(isLocalPathCandidate('https://example.com/shot.png')).toBe(false)
    expect(isLocalPathCandidate('prime://settings/general')).toBe(false)
    expect(isLocalPathCandidate('v1/2')).toBe(false)
    expect(isLocalPathCandidate('application/json')).toBe(false)
    expect(isExplicitRelativePathCandidate('./out/frames')).toBe(true)
    expect(isExplicitRelativePathCandidate('C:\\out\\frames')).toBe(true)
  })
  it('recognizes document and archive paths (absolute, relative, bare) while rejecting URLs, MIME types, and versions', () => {
    expect(isDocumentPath('report.pdf')).toBe(true)
    expect(isDocumentPath('proposal.docx')).toBe(true)
    expect(isDocumentPath('budget.xlsx')).toBe(true)
    expect(isDocumentPath('slides.pptx')).toBe(true)
    expect(isDocumentPath('notes.txt')).toBe(true)
    expect(isDocumentPath('README.md')).toBe(true)
    expect(isDocumentPath('data.csv')).toBe(true)
    expect(isDocumentPath('archive.zip')).toBe(true)
    expect(isDocumentPath('config.json')).toBe(true)
    expect(isDocumentPath('index.html')).toBe(true)
    expect(isDocumentPath('application/json')).toBe(false)

    expect(isLocalPathCandidate('C:\\docs\\report.pdf')).toBe(true)
    expect(isLocalPathCandidate('/Users/dev/proposal.docx')).toBe(true)
    expect(isLocalPathCandidate('./dir/file.pdf')).toBe(true)
    expect(isLocalPathCandidate('./sheets/budget.xlsx')).toBe(true)
    expect(isLocalPathCandidate('docs/deck.pptx')).toBe(true)
    expect(isLocalPathCandidate('report.pdf')).toBe(true)
    expect(isLocalPathCandidate('budget.xlsx')).toBe(true)
    expect(isLocalPathCandidate('Отчёт.docx')).toBe(true)

    expect(isLocalPathCandidate('https://example.com/report.pdf')).toBe(false)
    expect(isLocalPathCandidate('prime://settings/general')).toBe(false)
    expect(isLocalPathCandidate('application/json')).toBe(false)
    expect(isLocalPathCandidate('application/pdf')).toBe(false)
    expect(isLocalPathCandidate('text/html')).toBe(false)
    expect(isLocalPathCandidate('v1/2')).toBe(false)
    expect(isLocalPathCandidate('../secret.pdf')).toBe(false)
    expect(isLocalPathCandidate('dir/../secret.pdf')).toBe(false)
    expect(isLocalPathCandidate('./dir/../secret.pdf')).toBe(false)
    expect(isExplicitRelativePathCandidate('./dir/../secret.pdf')).toBe(false)
    expect(extractTextPathSegments('Не открывать dir/../secret.pdf.')).toEqual([
      { type: 'text', value: 'Не открывать dir/../secret.pdf.' }
    ])

    const line =
      'Готово: C:\\docs\\report.pdf, ./out/spec.docx, budget.xlsx и https://example.com/files/report.pdf (формат application/json, v1/2).'
    expect(extractTextPathSegments(line)).toEqual([
      { type: 'text', value: 'Готово: ' },
      { type: 'path', value: 'C:\\docs\\report.pdf' },
      { type: 'text', value: ', ' },
      { type: 'path', value: './out/spec.docx' },
      { type: 'text', value: ', ' },
      { type: 'path', value: 'budget.xlsx' },
      { type: 'text', value: ' и https://example.com/files/report.pdf (формат application/json, v1/2).' }
    ])
  })
})

describe('MarkdownText local path rendering', () => {
  it('renders standalone image, video and folder paths as dedicated cards', () => {
    seedChatPathInspectionCache('ws-1', 'C:\\shots\\bug.png', {
      exists: true,
      kind: 'image',
      mime: 'image/png',
      name: 'bug.png',
      path: 'C:\\shots\\bug.png',
      previewable: true,
      revealable: true
    })
    seedChatPathInspectionCache('ws-1', 'C:\\shots\\frames', {
      exists: true,
      kind: 'dir',
      name: 'frames',
      path: 'C:\\shots\\frames',
      previewable: false,
      revealable: true
    })
    seedChatPathInspectionCache('ws-1', 'out/run.mp4', {
      exists: true,
      kind: 'video',
      mime: 'video/mp4',
      name: 'run.mp4',
      path: 'out/run.mp4',
      previewable: true,
      revealable: true
    })

    const imageMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text="C:\shots\bug.png" />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(imageMarkup).toContain('chat-path-media-card is-image')
    expect(imageMarkup).toContain('bug.png')
    expect(imageMarkup).toContain('Открыть')
    expect(imageMarkup).not.toContain('<p>')

    const folderMarkup = renderToStaticMarkup(
      <I18nProvider language="en">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text="`C:\shots\frames`" />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(folderMarkup).toContain('chat-path-folder-card')
    expect(folderMarkup).toContain('frames')
    expect(folderMarkup).toContain('Open')
    expect(folderMarkup).not.toContain('<p>')

    const videoMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text="`out/run.mp4`" />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(videoMarkup).toContain('chat-path-media-card is-video')
    expect(videoMarkup).toContain('video-player')
    expect(videoMarkup).toContain('run.mp4')
  })

  it('renders inline paths inside text as chips and keeps fenced code blocks untouched', () => {
    seedChatPathInspectionCache('ws-1', 'C:\\shots\\frames', {
      exists: true,
      kind: 'dir',
      name: 'frames',
      path: 'C:\\shots\\frames',
      previewable: false,
      revealable: true
    })

    const inlineMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text="Кадры сохранены в C:\shots\frames для проверки." />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(inlineMarkup).toContain('<p>')
    expect(inlineMarkup).toContain('chat-path-chip is-dir')
    expect(inlineMarkup).toContain('frames')

    const fencedMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text={'```txt\nC:\\shots\\frames\n```'} />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(fencedMarkup).toContain('<pre>')
    expect(fencedMarkup).not.toContain('chat-path-folder-card')
    expect(fencedMarkup).not.toContain('chat-path-chip')
  })

  it('allows local MediaServer loopback URLs in img-src CSP for index.html and tauri.conf.json', () => {
    const indexHtml = readFileSync(resolve(__dirname, '../../src/renderer/index.html'), 'utf8')
    const tauriConf = readFileSync(resolve(__dirname, '../../src-tauri/tauri.conf.json'), 'utf8')
    expect(indexHtml).toMatch(/img-src[^;]*http:\/\/127\.0\.0\.1:\*/)
    expect(tauriConf).toMatch(/img-src[^;]*http:\/\/127\.0\.0\.1:\*/)
  })

  it('renders inline image file chip smoothly inside text paragraph', () => {
    seedChatPathInspectionCache('ws-1', 'baba.png', {
      exists: true,
      kind: 'image',
      name: 'baba.png',
      path: 'C:\\proj\\baba.png',
      previewable: true
    })
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text="Раскладка в `baba.png` для вырезки." />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(html).toContain('chat-path-inline-group')
    expect(html).toContain('chat-path-chip is-image')
    expect(html).toContain('baba.png')
  })

  it('renders standalone document paths as format-styled file cards with metadata and actions', () => {
    const cases = [
      {
        input: 'C:\\docs\\report.pdf',
        markdown: 'C:\\docs\\report.pdf',
        name: 'report.pdf',
        extension: 'pdf',
        size: 128 * 1024,
        tone: 'is-pdf',
        badge: 'PDF',
        ruSize: '128 КБ',
        enSize: '128 KB'
      },
      {
        input: './docs/proposal.docx',
        markdown: '`./docs/proposal.docx`',
        name: 'proposal.docx',
        extension: 'docx',
        size: 42 * 1024,
        tone: 'is-doc',
        badge: 'DOCX',
        ruSize: '42 КБ',
        enSize: '42 KB'
      },
      {
        input: 'budget.xlsx',
        markdown: '`budget.xlsx`',
        name: 'budget.xlsx',
        extension: 'xlsx',
        size: 1536 * 1024,
        tone: 'is-sheet',
        badge: 'XLSX',
        ruSize: '1.5 МБ',
        enSize: '1.5 MB'
      },
      {
        input: 'out/deck.pptx',
        markdown: 'out/deck.pptx',
        name: 'deck.pptx',
        extension: 'pptx',
        size: 860,
        tone: 'is-slides',
        badge: 'PPTX',
        ruSize: '860 Б',
        enSize: '860 B'
      },
      {
        input: 'archive.zip',
        markdown: '`archive.zip`',
        name: 'archive.zip',
        extension: 'zip',
        size: 2 * 1024 * 1024,
        tone: 'is-neutral',
        badge: 'ZIP',
        ruSize: '2 МБ',
        enSize: '2 MB'
      }
    ]

    for (const item of cases) {
      seedChatPathInspectionCache('ws-1', item.input, {
        exists: true,
        kind: 'file',
        name: item.name,
        path: `C:\\proj\\${item.name}`,
        extension: item.extension,
        size: item.size,
        previewable: false,
        revealable: true
      })

      const ruHtml = renderToStaticMarkup(
        <I18nProvider language="ru">
          <WorkspaceMediaProvider workspaceId="ws-1">
            <MarkdownText text={item.markdown} />
          </WorkspaceMediaProvider>
        </I18nProvider>
      )
      expect(ruHtml).toContain(`chat-path-file-card ${item.tone}`)
      expect(ruHtml).toContain(item.name)
      expect(ruHtml).toContain(item.badge)
      expect(ruHtml).toContain(item.ruSize)
      expect(ruHtml).toContain('Открыть')
      expect(ruHtml).toContain('Показать в папке')
      expect(ruHtml).not.toContain('<p>')

      const enHtml = renderToStaticMarkup(
        <I18nProvider language="en">
          <WorkspaceMediaProvider workspaceId="ws-1">
            <MarkdownText text={item.markdown} />
          </WorkspaceMediaProvider>
        </I18nProvider>
      )
      expect(enHtml).toContain(item.badge)
      expect(enHtml).toContain(item.enSize)
      expect(enHtml).toContain('Open')
      expect(enHtml).toContain('Show in folder')
    }
  })

  it('renders inline document paths inside prose as file chips inside inline groups', () => {
    seedChatPathInspectionCache('ws-1', 'report.pdf', {
      exists: true,
      kind: 'file',
      name: 'report.pdf',
      path: 'C:\\proj\\report.pdf',
      extension: 'pdf',
      size: 64 * 1024,
      previewable: false,
      revealable: true
    })
    seedChatPathInspectionCache('ws-1', './out/budget.xlsx', {
      exists: true,
      kind: 'file',
      name: 'budget.xlsx',
      path: 'C:\\proj\\out\\budget.xlsx',
      extension: 'xlsx',
      size: 24 * 1024,
      previewable: false,
      revealable: true
    })

    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text="Итоговый отчёт report.pdf и расчёт `./out/budget.xlsx` готовы." />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(html).toContain('<p>')
    expect(html).toContain('chat-path-inline-group')
    expect(html).toContain('chat-path-chip is-file is-pdf')
    expect(html).toContain('chat-path-chip is-file is-sheet')
    expect(html).toContain('chat-path-reveal-icon')
    expect(html).toContain('report.pdf')
    expect(html).toContain('budget.xlsx')
  })

  it('invokes openPath on primary open/chip click and reveal on folder icon click with safe fallback', () => {
    const openPath = vi.fn().mockResolvedValue(undefined)
    const reveal = vi.fn().mockResolvedValue(undefined)
    ;(globalThis as Record<string, unknown>).window = {
      prime: { openPath, reveal }
    }
    const tRu = (source: string, values?: Record<string, string | number>) =>
      translate('ru', source, values)

    const blockTree = ChatPathFileView({
      variant: 'block',
      workspaceId: 'ws-42',
      targetPath: 'C:\\proj\\report.pdf',
      displayName: 'report.pdf',
      extension: 'pdf',
      size: 128 * 1024,
      t: tRu
    })
    const blockButtons = findButtons(blockTree)
    expect(blockButtons).toHaveLength(2)
    blockButtons[0].props.onClick()
    expect(openPath).toHaveBeenCalledWith('ws-42', 'C:\\proj\\report.pdf')
    blockButtons[1].props.onClick()
    expect(reveal).toHaveBeenCalledWith('ws-42', 'C:\\proj\\report.pdf')

    openPath.mockClear()
    reveal.mockClear()

    const inlineTree = ChatPathFileView({
      variant: 'inline',
      workspaceId: 'ws-42',
      targetPath: 'C:\\proj\\budget.xlsx',
      displayName: 'budget.xlsx',
      extension: 'xlsx',
      size: 42 * 1024,
      t: tRu
    })
    const inlineButtons = findButtons(inlineTree)
    expect(inlineButtons).toHaveLength(2)
    inlineButtons[0].props.onClick()
    expect(openPath).toHaveBeenCalledWith('ws-42', 'C:\\proj\\budget.xlsx')
    inlineButtons[1].props.onClick()
    expect(reveal).toHaveBeenCalledWith('ws-42', 'C:\\proj\\budget.xlsx')

    reveal.mockClear()
    ;(globalThis as Record<string, unknown>).window = {
      prime: { reveal }
    }
    blockButtons[0].props.onClick()
    expect(reveal).toHaveBeenCalledWith('ws-42', 'C:\\proj\\report.pdf')
  })
})

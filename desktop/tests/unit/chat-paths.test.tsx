import { afterEach, describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { WorkspaceMediaProvider } from '../../src/renderer/src/features/chat/WorkspaceMediaContext'
import { MarkdownText } from '../../src/renderer/src/features/chat/MarkdownText'
import {
  clearChatPathInspectionCache,
  seedChatPathInspectionCache
} from '../../src/renderer/src/features/chat/ChatPathEntry'
import {
  extractTextPathSegments,
  isExplicitRelativePathCandidate,
  isLocalPathCandidate
} from '../../src/renderer/src/features/chat/chatPaths'

afterEach(() => {
  clearChatPathInspectionCache()
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

  it('rejects HTTP URLs, prime links, and version fractions', () => {
    expect(isLocalPathCandidate('https://example.com/shot.png')).toBe(false)
    expect(isLocalPathCandidate('prime://settings/general')).toBe(false)
    expect(isLocalPathCandidate('v1/2')).toBe(false)
    expect(isLocalPathCandidate('application/json')).toBe(false)
    expect(isExplicitRelativePathCandidate('./out/frames')).toBe(true)
    expect(isExplicitRelativePathCandidate('C:\\out\\frames')).toBe(true)
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
})

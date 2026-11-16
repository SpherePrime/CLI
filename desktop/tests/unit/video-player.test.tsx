import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { WorkspaceMediaProvider } from '../../src/renderer/src/features/chat/WorkspaceMediaContext'
import { MarkdownText } from '../../src/renderer/src/features/chat/MarkdownText'
import { MessageAttachment } from '../../src/renderer/src/features/chat/MessageAttachment'
import { MessageItem } from '../../src/renderer/src/features/chat/MessageItem'
import { VideoPlayer } from '../../src/renderer/src/features/chat/video/VideoPlayer'
import { VideoLightbox } from '../../src/renderer/src/features/chat/video/VideoLightbox'
import { collectToolVideos } from '../../src/renderer/src/features/chat/video/ToolVideos'
import {
  fileNameFromPath,
  formatVideoTime,
  isVideoExtension,
  isVideoMime,
  isVideoPath,
  isVideoUrl,
  videoMimeFromPath
} from '../../src/renderer/src/features/chat/video/videoMedia'
import type { Message } from '../../src/shared/contracts'

describe('videoMedia helpers', () => {
  it('detects supported video extensions, paths, URLs and MIME types', () => {
    expect(isVideoExtension('mp4')).toBe(true)
    expect(isVideoExtension('.WEBM')).toBe(true)
    expect(isVideoExtension('mov')).toBe(true)
    expect(isVideoExtension('m4v')).toBe(true)
    expect(isVideoExtension('ogv')).toBe(true)
    expect(isVideoExtension('png')).toBe(false)

    expect(isVideoPath('out/heart.mp4')).toBe(true)
    expect(isVideoPath('C:\\renders\\clip.MOV?v=1#t=2')).toBe(true)
    expect(isVideoPath('src/App.tsx')).toBe(false)

    expect(isVideoUrl('out/heart.mp4')).toBe(true)
    expect(isVideoUrl('https://cdn.example.com/demo.webm?token=abc#frag')).toBe(true)
    expect(isVideoUrl('data:video/mp4;base64,AAAA')).toBe(true)
    expect(isVideoUrl('C:\\Users\\demo\\out\\clip.m4v')).toBe(true)
    expect(isVideoUrl('data:image/png;base64,AAAA')).toBe(false)
    expect(isVideoUrl('https://example.com/article')).toBe(false)
    expect(isVideoUrl('prime://settings/appearance')).toBe(false)

    expect(isVideoMime('video/mp4')).toBe(true)
    expect(isVideoMime('video/webm; codecs=vp8')).toBe(true)
    expect(isVideoMime('image/png')).toBe(false)
  })

  it('resolves MIME types, filenames and timecodes', () => {
    expect(videoMimeFromPath('out/heart.mp4')).toBe('video/mp4')
    expect(videoMimeFromPath('out/clip.m4v')).toBe('video/mp4')
    expect(videoMimeFromPath('out/clip.webm?raw=1')).toBe('video/webm')
    expect(videoMimeFromPath('out/clip.mov')).toBe('video/quicktime')
    expect(videoMimeFromPath('out/clip.ogv')).toBe('video/ogg')
    expect(videoMimeFromPath('out/notes.txt')).toBeUndefined()

    expect(fileNameFromPath('out/heart.mp4?v=1')).toBe('heart.mp4')
    expect(fileNameFromPath('C:\\clips\\final%20cut.webm')).toBe('final cut.webm')

    expect(formatVideoTime(0)).toBe('0:00')
    expect(formatVideoTime(5)).toBe('0:05')
    expect(formatVideoTime(65)).toBe('1:05')
    expect(formatVideoTime(3661)).toBe('1:01:01')
    expect(formatVideoTime(-2)).toBe('0:00')
    expect(formatVideoTime(Number.NaN)).toBe('0:00')
  })
})

describe('MarkdownText video rendering', () => {
  it('renders video image embeds as a block VideoPlayer without wrapping <p>', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text="![clip](out/heart.mp4)" />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(markup).toContain('class="video-player variant-chat"')
    expect(markup).toContain('<video')
    expect(markup).toContain('clip')
    expect(markup).not.toContain('<img')
    expect(markup).not.toContain('<p>')
  })

  it('renders standalone video links as a block VideoPlayer', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text="[clip](out/heart.mp4)" />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(markup).toContain('class="video-player variant-chat"')
    expect(markup).toContain('<video')
    expect(markup).toContain('clip')
    expect(markup).not.toContain('<a ')
    expect(markup).not.toContain('<p>')
  })

  it('renders inline video links inside sentences as a lightbox trigger chip', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MarkdownText text="Смотри [clip](out/heart.mp4) для деталей" />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(markup).toContain('<p>')
    expect(markup).toContain('class="video-inline-chip"')
    expect(markup).toContain('clip')
    expect(markup).not.toContain('class="video-player')
    expect(markup).not.toContain('<a ')
  })

  it('keeps non-video markdown links as regular anchors', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <MarkdownText text="Открой [документацию](https://example.com/docs)" />
      </I18nProvider>
    )
    expect(markup).toContain('<a href="https://example.com/docs">документацию</a>')
    expect(markup).not.toContain('video-inline-chip')
  })
})

describe('VideoPlayer, VideoLightbox, MessageAttachment and MessageItem', () => {
  it('renders accessible transport controls and supports RU/EN localization', () => {
    const ruMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VideoPlayer src="data:video/webm;base64,AAAA" title="demo.webm" mime="video/webm" />
      </I18nProvider>
    )
    expect(ruMarkup).toContain('class="video-player variant-chat"')
    expect(ruMarkup).toContain('demo.webm')
    expect(ruMarkup).toContain('aria-label="Воспроизвести"')
    expect(ruMarkup).toContain('aria-label="Перемотка видео"')
    expect(ruMarkup).toContain('aria-label="Выключить звук"')
    expect(ruMarkup).toContain('aria-label="Скорость воспроизведения"')
    expect(ruMarkup).toContain('aria-label="Открыть видео на весь экран"')
    expect(ruMarkup).toContain('0:00 / 0:00')
    expect(ruMarkup).toContain('1×')

    const enMarkup = renderToStaticMarkup(
      <I18nProvider language="en">
        <VideoPlayer src="data:video/webm;base64,AAAA" title="demo.webm" mime="video/webm" />
      </I18nProvider>
    )
    expect(enMarkup).toContain('aria-label="Play"')
    expect(enMarkup).toContain('aria-label="Seek video"')
    expect(enMarkup).toContain('aria-label="Mute"')
    expect(enMarkup).toContain('aria-label="Playback speed"')
    expect(enMarkup).toContain('aria-label="Open video fullscreen"')
  })

  it('hides expand button inside lightbox and renders error retry state', () => {
    const lightboxMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VideoLightbox src="data:video/webm;base64,AAAA" title="demo.webm" close={() => {}} />
      </I18nProvider>
    )
    expect(lightboxMarkup).toContain('class="video-lightbox"')
    expect(lightboxMarkup).toContain('class="video-player variant-lightbox"')
    expect(lightboxMarkup).toContain('aria-label="Закрыть видео"')
    expect(lightboxMarkup).not.toContain('Открыть видео на весь экран')

    const errorMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <VideoPlayer src="data:video/webm;base64,AAAA" title="broken.webm" error />
      </I18nProvider>
    )
    expect(errorMarkup).toContain('Не удалось загрузить видео')
    expect(errorMarkup).toContain('Повторить')
  })

  it('renders video attachments with base64 content or workspace path', () => {
    const base64Markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <MessageAttachment
          data={{ mime_type: 'video/mp4', data: 'AAAA', path: 'out/render.mp4' }}
        />
      </I18nProvider>
    )
    expect(base64Markup).toContain('class="video-player variant-chat"')
    expect(base64Markup).toContain('src="data:video/mp4;base64,AAAA"')
    expect(base64Markup).toContain('render.mp4')

    const pathMarkup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MessageAttachment data={{ mime_type: 'video/mp4', path: 'out/demo.mp4' }} />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(pathMarkup).toContain('class="video-player variant-chat"')
    expect(pathMarkup).toContain('demo.mp4')
  })

  it('renders video image_url parts and tool result videos in MessageItem', () => {
    expect(
      collectToolVideos({
        mime_type: 'video/mp4',
        data: 'AAA',
        videos: [
          { mime_type: 'video/mp4', data: 'AAA' },
          { mime_type: 'video/webm', path: 'out/second.webm' }
        ]
      })
    ).toHaveLength(2)

    const message: Message = {
      id: 'm-1',
      session_id: 's-1',
      role: 'assistant',
      parts: [
        { type: 'image_url', data: { url: 'data:video/webm;base64,AAAA' } },
        {
          type: 'tool_call',
          data: {
            id: 'tc-1',
            name: 'render_video',
            input: {},
            result: {
              content: 'Rendered out/demo.mp4',
              video: { path: 'out/demo.mp4', mime_type: 'video/mp4' }
            }
          }
        }
      ]
    }
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkspaceMediaProvider workspaceId="ws-1">
          <MessageItem
            message={message}
            continued={false}
            fresh={false}
            live={false}
            matched={false}
          />
        </WorkspaceMediaProvider>
      </I18nProvider>
    )
    expect(markup).toContain('src="data:video/webm;base64,AAAA"')
    expect(markup).toContain('class="tool-videos"')
    expect(markup).toContain('demo.mp4')
  })
})

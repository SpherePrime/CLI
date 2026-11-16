import { useI18n } from '../../../i18n'
import { VideoPlayer } from './VideoPlayer'
import { WorkspaceVideo } from './WorkspaceVideo'
import { fileNameFromPath, isDirectMediaUrl, isVideoMime, isVideoUrl, videoMimeFromPath } from './videoMedia'

export interface ToolVideo {
  data?: string
  path?: string
  url?: string
  mime_type?: string
  title?: string
}

function normalizeVideoEntry(entry: unknown): ToolVideo | null {
  if (typeof entry === 'string') {
    const trimmed = entry.trim()
    if (!trimmed || (!isVideoUrl(trimmed) && !isDirectMediaUrl(trimmed))) return null
    return isDirectMediaUrl(trimmed)
      ? { url: trimmed, mime_type: videoMimeFromPath(trimmed), title: fileNameFromPath(trimmed) }
      : { path: trimmed, mime_type: videoMimeFromPath(trimmed), title: fileNameFromPath(trimmed) }
  }
  if (!entry || typeof entry !== 'object') return null
  const record = entry as Record<string, unknown>
  const mime =
    typeof record.mime_type === 'string'
      ? record.mime_type
      : typeof record.mimeType === 'string'
        ? record.mimeType
        : typeof record.mime === 'string'
          ? record.mime
          : ''
  const data = typeof record.data === 'string' && record.data ? record.data : undefined
  const path = typeof record.path === 'string' && record.path ? record.path : undefined
  const url = typeof record.url === 'string' && record.url ? record.url : undefined
  const title =
    typeof record.title === 'string' && record.title
      ? record.title
      : typeof record.name === 'string' && record.name
        ? record.name
        : undefined
  if (!data && !path && !url) return null
  if (mime && !isVideoMime(mime)) return null
  return {
    data,
    path,
    url,
    mime_type: mime || (path ? videoMimeFromPath(path) : url ? videoMimeFromPath(url) : undefined),
    title
  }
}

export function collectToolVideos(result: unknown): ToolVideo[] {
  if (!result || typeof result !== 'object') return []
  const record = result as Record<string, unknown>
  const videos: ToolVideo[] = []
  const seen = new Set<string>()

  const pushUnique = (item: ToolVideo | null) => {
    if (!item) return
    const key = `${item.data || ''}|${item.path || ''}|${item.url || ''}`
    if (seen.has(key)) return
    seen.add(key)
    videos.push(item)
  }

  const rootMime =
    typeof record.mime_type === 'string'
      ? record.mime_type
      : typeof record.mimeType === 'string'
        ? record.mimeType
        : ''
  if (isVideoMime(rootMime)) {
    pushUnique(
      normalizeVideoEntry({
        data: record.data,
        path: record.path,
        url: record.url,
        mime_type: rootMime,
        title: record.title ?? record.name
      })
    )
  }

  if (record.video) {
    pushUnique(normalizeVideoEntry(record.video))
  }

  if (Array.isArray(record.videos)) {
    for (const item of record.videos) {
      pushUnique(normalizeVideoEntry(item))
    }
  }

  return videos
}

export function ToolVideos({ result }: { result: unknown }) {
  const { t } = useI18n()
  const videos = collectToolVideos(result)
  if (!videos.length) return null

  return (
    <div className="tool-videos">
      {videos.map((video, index) => {
        const title =
          video.title ||
          fileNameFromPath(video.path || video.url || '') ||
          t('Видео')
        if (video.data) {
          const mime = video.mime_type || 'video/mp4'
          return (
            <VideoPlayer
              key={index}
              src={`data:${mime};base64,${video.data}`}
              title={title}
              mime={mime}
              variant="chat"
            />
          )
        }
        const source = video.url || video.path || ''
        if (!source) return null
        return (
          <WorkspaceVideo
            key={index}
            src={source}
            title={title}
            mime={video.mime_type}
            variant="chat"
          />
        )
      })}
    </div>
  )
}

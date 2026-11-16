const VIDEO_EXTENSIONS = new Set(['mp4', 'webm', 'mov', 'm4v', 'ogv'])

const MIME_BY_EXTENSION: Record<string, string> = {
  mp4: 'video/mp4',
  m4v: 'video/mp4',
  webm: 'video/webm',
  mov: 'video/quicktime',
  ogv: 'video/ogg'
}

const FORMAT_BY_MIME: Record<string, string> = {
  'video/mp4': 'MP4',
  'video/webm': 'WEBM',
  'video/quicktime': 'MOV',
  'video/x-m4v': 'M4V',
  'video/ogg': 'OGV'
}

function stripQueryAndHash(value: string): string {
  const queryIndex = value.indexOf('?')
  const hashIndex = value.indexOf('#')
  let end = value.length
  if (queryIndex !== -1) end = Math.min(end, queryIndex)
  if (hashIndex !== -1) end = Math.min(end, hashIndex)
  return value.slice(0, end)
}

function extensionFromPath(path: string): string {
  const cleaned = stripQueryAndHash(path.trim())
  const fileName = cleaned.split(/[\\/]/).pop() || ''
  const dotIndex = fileName.lastIndexOf('.')
  if (dotIndex <= 0 || dotIndex === fileName.length - 1) return ''
  return fileName.slice(dotIndex + 1).toLowerCase()
}

export function isVideoExtension(extension: string): boolean {
  if (typeof extension !== 'string') return false
  const normalized = extension.trim().replace(/^\./, '').toLowerCase()
  return VIDEO_EXTENSIONS.has(normalized)
}

export function isVideoPath(path: string): boolean {
  if (typeof path !== 'string' || !path.trim()) return false
  return isVideoExtension(extensionFromPath(path))
}

export function isVideoUrl(url: string): boolean {
  if (typeof url !== 'string') return false
  const trimmed = url.trim()
  if (!trimmed) return false
  if (/^data:video\//i.test(trimmed)) return true
  if (/^(data:|prime:|mailto:|tel:|javascript:)/i.test(trimmed)) return false
  if (/^https?:\/\//i.test(trimmed)) {
    try {
      return isVideoPath(new URL(trimmed).pathname)
    } catch {
      return isVideoPath(trimmed)
    }
  }
  return isVideoPath(trimmed)
}

export function isVideoMime(mime: string): boolean {
  if (typeof mime !== 'string') return false
  return /^video\//i.test(mime.trim())
}

export function videoMimeFromPath(path: string): string | undefined {
  if (typeof path !== 'string' || !path.trim()) return undefined
  const ext = extensionFromPath(path)
  return MIME_BY_EXTENSION[ext]
}

export function fileNameFromPath(path: string): string {
  if (typeof path !== 'string') return ''
  const cleaned = stripQueryAndHash(path.trim())
  if (!cleaned) return ''
  const parts = cleaned.split(/[\\/]/).filter(Boolean)
  const last = parts[parts.length - 1] || ''
  try {
    return decodeURIComponent(last)
  } catch {
    return last
  }
}

export function formatVideoTime(seconds: number): string {
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds < 0) {
    return '0:00'
  }
  const totalSeconds = Math.floor(seconds)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const secs = totalSeconds % 60
  const paddedSecs = String(secs).padStart(2, '0')
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, '0')}:${paddedSecs}`
  }
  return `${minutes}:${paddedSecs}`
}

export function videoFormatLabel(mime?: string, path?: string): string {
  const ext = path ? extensionFromPath(path) : ''
  if (ext && VIDEO_EXTENSIONS.has(ext)) {
    return ext.toUpperCase()
  }
  if (mime) {
    const normalized = mime.split(';')[0].trim().toLowerCase()
    if (FORMAT_BY_MIME[normalized]) return FORMAT_BY_MIME[normalized]
    if (normalized.startsWith('video/')) {
      return normalized.slice(6).toUpperCase()
    }
  }
  return ''
}

export function isDirectMediaUrl(url: string): boolean {
  return /^(https?:|data:|blob:)/i.test(url.trim())
}

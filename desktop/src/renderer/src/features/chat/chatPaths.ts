import { isVideoPath } from './video/videoMedia'

const IMAGE_EXTENSIONS = new Set(['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp', 'avif'])
const DOCUMENT_EXTENSIONS = new Set([
  'pdf',
  'docx',
  'doc',
  'xlsx',
  'xls',
  'csv',
  'pptx',
  'ppt',
  'txt',
  'md',
  'rtf',
  'odt',
  'ods',
  'odp',
  'zip',
  'json',
  'html'
])
const SCHEME_PREFIX = /^(https?:|prime:|prime-path:|mailto:|tel:|data:|blob:|javascript:)/i
const PARENT_TRAVERSAL_PATTERN = /(?:^|[\\/])\.\.(?:[\\/]|$)/
const NON_PATH_SLASH_PATTERN =
  /^(?:v?\d+[\\/]\d+$|application[\\/]|text[\\/]|image[\\/]|video[\\/]|audio[\\/]|multipart[\\/]|font[\\/])/i
const TRAILING_PUNCTUATION = /[.,;:!?)\]>'"»]+$/

const BARE_FILE_EXTENSIONS =
  'pdf|docx?|xlsx?|csv|pptx?|txt|md|rtf|od[tsp]|zip|json|html|png|jpe?g|webp|gif|bmp|avif|mp4|webm|mov|m4v|ogv'

const PATH_PATTERN = new RegExp(
  `(file:\\/\\/[^\\s<>"'\`]+|[a-zA-Z]:[\\\\/][^\\s<>"'|?*\`]+|\\\\\\\\[^\\s\\\\/]+[\\\\/][^\\s<>"'|?*\`]+|\\/(?:tmp|var|Users|home|opt|mnt|Volumes|srv|root)[\\\\/][^\\s<>"'|?*\`]+|(?:\\.[\\\\/])?(?:[\\p{L}\\p{N}_.-]+[\\\\/])+[\\p{L}\\p{N}_.-]*[\\\\/]?|[\\p{L}\\p{N}_-]+(?:\\.[\\p{L}\\p{N}_-]+)*\\.(?:${BARE_FILE_EXTENSIONS})(?=$|[\\s.,;:!?)\\]>'"»\`]))`,
  'giu'
)

export const CHAT_PATH_LINK_PREFIX = 'prime-path:'

export interface TextPathSegment {
  type: 'text' | 'path'
  value: string
}

interface MdastNode {
  type: string
  value?: string
  url?: string
  children?: MdastNode[]
  data?: Record<string, unknown>
}

function stripTrailingPunctuation(raw: string): { path: string; suffix: string } {
  const match = TRAILING_PUNCTUATION.exec(raw)
  if (!match) return { path: raw, suffix: '' }
  const cut = raw.length - match[0].length
  return { path: raw.slice(0, cut), suffix: raw.slice(cut) }
}

function isPrecededByBoundary(text: string, index: number): boolean {
  if (index <= 0) return true
  const prev = text[index - 1]
  return /[\s(\[{"'`:=«]/.test(prev)
}

function isInsideUrl(text: string, index: number, matchLength: number): boolean {
  let tokenStart = index
  while (tokenStart > 0 && !/[\s(\[{"'`«<>]/.test(text[tokenStart - 1])) {
    tokenStart -= 1
  }
  const token = text.slice(tokenStart, index + matchLength)
  if (/^file:\/\//i.test(token)) return false
  if (SCHEME_PREFIX.test(token)) return true
  return /[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(token)
}

export function getPathExtension(path: string): string {
  if (typeof path !== 'string' || !path.trim()) return ''
  const cleaned = path.split(/[?#]/)[0].trim()
  const fileName = cleaned.split(/[\\/]/).pop() || ''
  const dotIndex = fileName.lastIndexOf('.')
  if (dotIndex <= 0 || dotIndex === fileName.length - 1) return ''
  return fileName.slice(dotIndex + 1).toLowerCase()
}

export function isImagePath(path: string): boolean {
  return IMAGE_EXTENSIONS.has(getPathExtension(path))
}

export function isDocumentPath(path: string): boolean {
  return DOCUMENT_EXTENSIONS.has(getPathExtension(path))
}

export function isLocalPathCandidate(value: string): boolean {
  if (typeof value !== 'string') return false
  const trimmed = value.trim()
  if (!trimmed || trimmed.length > 500 || /[\r\n]/.test(trimmed)) return false
  if (PARENT_TRAVERSAL_PATTERN.test(trimmed)) return false
  if (/^file:\/\//i.test(trimmed)) return true
  if (SCHEME_PREFIX.test(trimmed)) return false
  if (/^[a-zA-Z]:[\\/].+/.test(trimmed)) return true
  if (/^\\\\[^\\/\s]+[\\/][^\\/\s]+/.test(trimmed)) return true
  if (/^\/(?:tmp|var|Users|home|opt|mnt|Volumes|srv|root)[\\/].+/.test(trimmed)) return true
  if (isImagePath(trimmed) || isVideoPath(trimmed) || isDocumentPath(trimmed)) {
    return !/[<>|?*]/.test(trimmed)
  }
  if (/^\.[\\/](?:[\p{L}\p{N}_.-]+[\\/]?)+$/u.test(trimmed)) {
    return !/[<>|?*]/.test(trimmed)
  }
  if (/^(?:[\p{L}\p{N}_.-]+[\\/])+[\p{L}\p{N}_.-]*[\\/]?$/u.test(trimmed)) {
    return !NON_PATH_SLASH_PATTERN.test(trimmed)
  }
  return false
}

export function isExplicitRelativePathCandidate(value: string): boolean {
  if (typeof value !== 'string') return false
  const trimmed = value.trim()
  if (PARENT_TRAVERSAL_PATTERN.test(trimmed)) return false
  if (/^\.[\\/](?:[\p{L}\p{N}_.-]+[\\/]?)+$/u.test(trimmed)) {
    return !/[<>|?*\r\n]/.test(trimmed)
  }
  return isLocalPathCandidate(trimmed)
}

export function extractTextPathSegments(text: string): TextPathSegment[] {
  if (!text) return []
  const segments: TextPathSegment[] = []
  let cursor = 0
  PATH_PATTERN.lastIndex = 0
  let match: RegExpExecArray | null
  while ((match = PATH_PATTERN.exec(text)) !== null) {
    const start = match.index
    if (!isPrecededByBoundary(text, start)) continue
    if (!/^file:\/\//i.test(match[0]) && isInsideUrl(text, start, match[0].length)) continue
    const { path, suffix } = stripTrailingPunctuation(match[0])
    if (!path || !isExplicitRelativePathCandidate(path)) continue
    if (start > cursor) {
      segments.push({ type: 'text', value: text.slice(cursor, start) })
    }
    segments.push({ type: 'path', value: path })
    cursor = start + match[0].length - suffix.length
  }
  if (cursor < text.length) {
    segments.push({ type: 'text', value: text.slice(cursor) })
  }
  return segments.length ? segments : [{ type: 'text', value: text }]
}

export function encodeChatPathHref(path: string): string {
  return `${CHAT_PATH_LINK_PREFIX}${encodeURIComponent(path)}`
}

export function decodeChatPathHref(href: string): string | null {
  if (!href.startsWith(CHAT_PATH_LINK_PREFIX)) return null
  try {
    return decodeURIComponent(href.slice(CHAT_PATH_LINK_PREFIX.length))
  } catch {
    return href.slice(CHAT_PATH_LINK_PREFIX.length)
  }
}

export function isLocalHrefTarget(href: string): boolean {
  if (!href) return false
  if (href.startsWith(CHAT_PATH_LINK_PREFIX)) return true
  if (SCHEME_PREFIX.test(href) && !/^file:\/\//i.test(href)) return false
  return isExplicitRelativePathCandidate(href) || /^file:\/\//i.test(href)
}

function transformMdastChildren(parent: MdastNode) {
  if (!Array.isArray(parent.children)) return
  const nextChildren: MdastNode[] = []
  for (const child of parent.children) {
    if (child.type === 'code' || child.type === 'link' || child.type === 'image' || child.type === 'html') {
      nextChildren.push(child)
      continue
    }
    if (child.type === 'inlineCode' && typeof child.value === 'string') {
      const candidate = child.value.trim()
      if (isExplicitRelativePathCandidate(candidate)) {
        nextChildren.push({
          type: 'link',
          url: encodeChatPathHref(candidate),
          data: { hProperties: { 'data-inline-code': 'true' } },
          children: [child]
        })
      } else {
        nextChildren.push(child)
      }
      continue
    }
    if (child.type === 'text' && typeof child.value === 'string') {
      const segments = extractTextPathSegments(child.value)
      for (const segment of segments) {
        if (segment.type === 'text') {
          nextChildren.push({ type: 'text', value: segment.value })
        } else {
          nextChildren.push({
            type: 'link',
            url: encodeChatPathHref(segment.value),
            children: [{ type: 'text', value: segment.value }]
          })
        }
      }
      continue
    }
    transformMdastChildren(child)
    nextChildren.push(child)
  }
  parent.children = nextChildren
}

export function remarkChatPaths() {
  return (tree: MdastNode) => {
    transformMdastChildren(tree)
  }
}

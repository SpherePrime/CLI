import type { Message, Project } from '../../../../shared/contracts'
import { textContent } from '../../../../shared/messages'
import { collectToolImages } from '../chat/ToolGallery'
import { isConversationPrompt } from '../chat/conversationTiming'

export type DesignImage = {
  name: string
  mimeType: string
  base64: string
}

export type DesignPayload = {
  title: string
  text: string
  images: DesignImage[]
}

const dataImageUrlPattern = /^data:(image\/[a-z0-9.+-]+);base64,(.+)$/i
const supportedUploadMimePattern = /^image\/(png|jpeg|jpg|gif|webp)$/i

export function isRegularProject(project: Project | null | undefined): project is Project {
  return Boolean(project && project.kind !== 'chat' && project.kind !== 'designer')
}

export function filterRegularProjects(projects: Project[]): Project[] {
  return projects.filter((project) => isRegularProject(project))
}

export function filterRegularRecentPaths(recent: string[], projects: Project[]): string[] {
  const knownPaths = new Set(projects.map((project) => project.path))
  return recent.filter((path) => path && path !== 'chat' && !knownPaths.has(path))
}

function extensionForMime(mimeType: string): string {
  const subtype = mimeType.toLowerCase().split('/')[1]?.split('+')[0]
  if (subtype === 'jpeg') return 'jpg'
  if (subtype === 'png' || subtype === 'gif' || subtype === 'webp' || subtype === 'svg') {
    return subtype
  }
  return 'png'
}

function normalizeImageEntry(
  rawData: string,
  rawMimeType: string,
  rawPath: string | undefined,
  index: number
): DesignImage | null {
  const trimmed = rawData.trim()
  if (!trimmed) return null
  const dataUrlMatch = dataImageUrlPattern.exec(trimmed)
  const mimeType = (dataUrlMatch ? dataUrlMatch[1] : rawMimeType).toLowerCase()
  const base64 = (dataUrlMatch ? dataUrlMatch[2] : trimmed).replace(/\s+/g, '')
  if (!mimeType.startsWith('image/') || !base64) return null
  const fileName = rawPath?.split(/[\\/]/).pop()?.trim()
  const ext = extensionForMime(mimeType)
  return {
    name: fileName || `design-mockup-${index + 1}.${ext}`,
    mimeType,
    base64
  }
}

function collectTurnMessages(
  message: Message,
  allMessages?: Message[]
): { turnMessages: Message[]; promptMessage?: Message } {
  if (!allMessages?.length) return { turnMessages: [message] }
  const targetIndex = allMessages.findIndex((item) => item.id === message.id)
  if (targetIndex < 0) return { turnMessages: [message] }

  const turnMessages: Message[] = []
  let promptMessage: Message | undefined
  for (let index = targetIndex; index >= 0; index--) {
    const candidate = allMessages[index]
    if (isConversationPrompt(candidate)) {
      promptMessage = candidate
      break
    }
    if (candidate.role === 'user') continue
    turnMessages.unshift(candidate)
  }
  if (!turnMessages.length) turnMessages.push(message)
  return { turnMessages, promptMessage }
}

function cleanDesignTitle(raw: string): string {
  const cleaned = raw
    .replace(/^#+\s*/, '')
    .replace(/^[-*•\d.)]+\s*/, '')
    .replace(/[*_`~]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
  if (!cleaned) return ''
  if (cleaned.length <= 60) return cleaned
  return `${cleaned.slice(0, 57).trimEnd()}…`
}

function deriveDesignTitle(text: string, promptMessage?: Message): string {
  const candidates = text
    .split(/\r?\n/)
    .map((line) => cleanDesignTitle(line))
    .filter((line) => line.length >= 3)
  if (candidates[0]) return candidates[0]

  if (promptMessage) {
    const promptLines = textContent(promptMessage)
      .split(/\r?\n/)
      .map((line) => cleanDesignTitle(line))
      .filter((line) => line.length >= 3)
    if (promptLines[0]) return promptLines[0]
  }
  return 'Реализация дизайна'
}

export function extractDesignFromMessage(
  message: Message,
  allMessages?: Message[]
): DesignPayload {
  const { turnMessages, promptMessage } = collectTurnMessages(message, allMessages)
  const textBlocks: string[] = []
  const images: DesignImage[] = []
  const seenImages = new Set<string>()

  const pushImage = (rawData: string, rawMimeType: string, rawPath?: string) => {
    const normalized = normalizeImageEntry(rawData, rawMimeType, rawPath, images.length)
    if (!normalized || seenImages.has(normalized.base64)) return
    seenImages.add(normalized.base64)
    images.push(normalized)
  }

  for (const item of turnMessages) {
    const text = textContent(item).trim()
    if (text) textBlocks.push(text)

    for (const part of item.parts || []) {
      if (part.type === 'tool_call' || part.type === 'tool_result') {
        const result = part.type === 'tool_result' ? part.data : part.data?.result
        for (const image of collectToolImages(result)) {
          pushImage(image.data, image.mime_type, image.path)
        }
        continue
      }
      if (part.type === 'binary') {
        const mime = part.data?.mime_type ?? part.data?.MIMEType ?? ''
        const data = part.data?.data ?? part.data?.Data ?? ''
        const path = part.data?.path ?? part.data?.Path
        if (typeof mime === 'string' && typeof data === 'string') {
          pushImage(data, mime, typeof path === 'string' ? path : undefined)
        }
        continue
      }
      if (part.type === 'image_url') {
        const url = part.data?.url
        if (typeof url === 'string' && url.startsWith('data:')) {
          pushImage(url, '')
        }
      }
    }
  }

  const combinedText = textBlocks.join('\n\n').trim()
  return {
    title: deriveDesignTitle(combinedText, promptMessage),
    text: combinedText,
    images
  }
}

export function formatDesignPreviewText(text: string): string {
  return text
    .replace(/```[\s\S]*?```/g, ' ')
    .split(/\r?\n/)
    .map((line) =>
      line
        .replace(/^\s*#{1,6}\s+/, '')
        .replace(/^\s*>\s?/, '')
        .replace(/^\s*[-*+]\s+/, '')
        .replace(/^\s*\d+\.\s+/, '')
        .replace(/\*\*([^*]+)\*\*/g, '$1')
        .replace(/__([^_]+)__/g, '$1')
        .replace(/`([^`]+)`/g, '$1')
        .trim()
    )
    .filter(Boolean)
    .join(' · ')
    .replace(/\s+/g, ' ')
    .trim()
}

export function buildDesignImplementationPrompt(design: DesignPayload): string {
  const sections: string[] = [
    'Реализуй в этом проекте утверждённый дизайн из вкладки «Дизайнер».',
    'Изучи текущую структуру проекта, используй принятые в нём стек, токены и компоненты, сохрани адаптивность и чистоту состояний.'
  ]

  if (design.images.length > 0) {
    sections.push(
      design.images.length === 1
        ? 'К этому сообщению прикреплён утверждённый визуальный макет дизайна — ориентируйся на его композицию, цвета, иерархию и акценты.'
        : `К этому сообщению прикреплены утверждённые визуальные макеты (${design.images.length} шт.) — ориентируйся на их композицию, цвета, иерархию и акценты.`
    )
  }

  if (design.text.trim()) {
    sections.push(`Спецификация дизайна:\n\n${design.text.trim()}`)
  }

  return sections.join('\n\n')
}

export function isSupportedUploadImage(image: DesignImage): boolean {
  return supportedUploadMimePattern.test(image.mimeType)
}

export function base64ToUint8Array(base64: string): Uint8Array {
  const normalized = base64.replace(/-/g, '+').replace(/_/g, '/').replace(/\s+/g, '')
  if (!normalized) return new Uint8Array(0)
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
  const binary =
    typeof atob === 'function'
      ? atob(padded)
      : Buffer.from(padded, 'base64').toString('binary')
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index++) {
    bytes[index] = binary.charCodeAt(index)
  }
  return bytes
}

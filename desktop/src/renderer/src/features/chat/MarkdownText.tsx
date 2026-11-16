import { isValidElement, memo, useMemo, type ReactNode } from 'react'
import Markdown, { defaultUrlTransform, type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { splitMarkdownBlocks } from './markdownBlocks'
import { WorkspaceVideo } from './video/WorkspaceVideo'
import { isVideoUrl } from './video/videoMedia'

type HastNode = {
  type?: string
  tagName?: string
  value?: string
  properties?: Record<string, unknown>
  children?: HastNode[]
}

function hastText(node: HastNode): string {
  if (node.type === 'text' && typeof node.value === 'string') return node.value
  if (!Array.isArray(node.children)) return ''
  return node.children.map(hastText).join('')
}

function reactNodeText(children: ReactNode): string {
  if (typeof children === 'string' || typeof children === 'number') return String(children)
  if (Array.isArray(children)) return children.map(reactNodeText).join('')
  if (isValidElement<{ children?: ReactNode }>(children)) {
    return reactNodeText(children.props.children)
  }
  return ''
}

function markdownUrlTransform(url: string): string {
  if (/^(prime:\/\/|data:video\/|[a-zA-Z]:[\\/])/.test(url)) return url
  return defaultUrlTransform(url)
}

const remarkPlugins = [remarkGfm]
const markdownComponents: Components = {
  p: ({ node, children }) => {
    const hast = node as HastNode | undefined
    const meaningful = Array.isArray(hast?.children)
      ? hast.children.filter((child) => !(child.type === 'text' && !child.value?.trim()))
      : []
    if (meaningful.length === 1) {
      const only = meaningful[0]
      if (only.type === 'element' && only.tagName === 'img') {
        const src = typeof only.properties?.src === 'string' ? only.properties.src : ''
        if (isVideoUrl(src)) {
          const alt = typeof only.properties?.alt === 'string' ? only.properties.alt : undefined
          return <WorkspaceVideo src={src} title={alt || undefined} variant="chat" />
        }
      }
      if (only.type === 'element' && only.tagName === 'a') {
        const href = typeof only.properties?.href === 'string' ? only.properties.href : ''
        if (isVideoUrl(href)) {
          const label = hastText(only).trim()
          const title = label && label !== href ? label : undefined
          return <WorkspaceVideo src={href} title={title} variant="chat" />
        }
      }
    }
    return <p>{children}</p>
  },
  img: ({ src, alt, node: _node, ...props }) => {
    if (typeof src === 'string' && isVideoUrl(src)) {
      return <WorkspaceVideo src={src} title={alt || undefined} variant="chat" />
    }
    return <img src={src} alt={alt} {...props} />
  },
  a: ({ href, children }) => {
    if (href?.startsWith('prime://')) {
      return (
        <a
          href={href}
          onClick={(event) => {
            event.preventDefault()
            const settingsMatch = /^prime:\/\/settings\/(.+)$/.exec(href)
            if (settingsMatch) {
              window.dispatchEvent(
                new CustomEvent('prime:open-settings', { detail: { tab: settingsMatch[1] } })
              )
            }
          }}
        >
          {children}
        </a>
      )
    }
    if (typeof href === 'string' && isVideoUrl(href)) {
      const label = reactNodeText(children).trim()
      const title = label && label !== href ? label : undefined
      return (
        <WorkspaceVideo src={href} title={title} inline>
          {children}
        </WorkspaceVideo>
      )
    }
    return (
      <a
        href={href}
        onClick={(event) => {
          event.preventDefault()
          if (href) void window.prime.openExternal(href)
        }}
      >
        {children}
      </a>
    )
  }
}

const MarkdownBlock = memo(function MarkdownBlock({ text }: { text: string }) {
  return (
    <Markdown
      remarkPlugins={remarkPlugins}
      components={markdownComponents}
      urlTransform={markdownUrlTransform}
    >
      {text}
    </Markdown>
  )
})

export const MarkdownText = memo(function MarkdownText({ text }: { text: string }) {
  const blocks = useMemo(() => splitMarkdownBlocks(text), [text])
  return blocks.map((block, index) => <MarkdownBlock key={index} text={block} />)
})

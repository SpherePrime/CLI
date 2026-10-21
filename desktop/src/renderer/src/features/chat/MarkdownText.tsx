import { memo, useMemo } from 'react'
import Markdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { splitMarkdownBlocks } from './markdownBlocks'

const remarkPlugins = [remarkGfm]
const markdownComponents: Components = {
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
    <Markdown remarkPlugins={remarkPlugins} components={markdownComponents}>
      {text}
    </Markdown>
  )
})

export const MarkdownText = memo(function MarkdownText({ text }: { text: string }) {
  const blocks = useMemo(() => splitMarkdownBlocks(text), [text])
  return blocks.map((block, index) => <MarkdownBlock key={index} text={block} />)
})

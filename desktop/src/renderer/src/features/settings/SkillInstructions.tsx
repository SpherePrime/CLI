import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Modal } from '../../components/Modal'

export function SkillInstructions({
  name,
  content,
  close
}: {
  name: string
  content: string
  close: () => void
}) {
  return (
    <Modal title={name} close={close} wide className="skill-instructions-modal">
      <div className="markdown skill-instructions">
        <Markdown
          remarkPlugins={[remarkGfm]}
          components={{
            a: ({ href, children }) => (
              <a
                href={href}
                onClick={(event) => {
                  event.preventDefault()
                  if (href?.startsWith('http')) void window.prime.openExternal(href)
                }}
              >
                {children}
              </a>
            )
          }}
        >
          {content.replace(/^---\n[\s\S]*?\n---\n/, '')}
        </Markdown>
      </div>
    </Modal>
  )
}

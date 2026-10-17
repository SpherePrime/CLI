import { useState } from 'react'
import { normalizeCommand, renderCommand } from '../../../../shared/commands'
import { Modal } from '../../components/Modal'
import type { Workspace } from '../../app/useWorkspace'
export function CommandRunner({
  command: raw,
  workspace: w,
  close,
  onReady
}: {
  command: any
  workspace: Workspace
  close: () => void
  onReady: (prompt: string) => void
}) {
  const command = normalizeCommand(raw)
  const [args, setArgs] = useState<Record<string, string>>({})
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  return (
    <Modal title={command.name} close={close}>
      <form
        onSubmit={async (event) => {
          event.preventDefault()
          setLoading(true)
          try {
            let prompt = renderCommand(command.content, args)
            if (command.clientId && command.promptId)
              prompt = (
                await w.call('mcpPrompt', {
                  client_id: command.clientId,
                  prompt_id: command.promptId,
                  args
                })
              ).prompt
            if (command.skill) {
              const data = await w.call('readSkill', {
                id: command.skill.SkillFilePath || command.skill.skill_file_path || command.skill.ID
              })
              const decoded = data.content
                ? new TextDecoder().decode(
                    Uint8Array.from(atob(data.content), (char) => char.charCodeAt(0))
                  )
                : ''
              prompt = `${command.name}\n\n${decoded}`
            }
            if (!prompt.trim()) throw new Error('Команда вернула пустой запрос')
            onReady(prompt)
            close()
          } catch (error: any) {
            setError(error.message)
          } finally {
            setLoading(false)
          }
        }}
      >
        {command.arguments.length ? (
          command.arguments.map((argument) => (
            <label className="form-label" key={argument.id}>
              {argument.title}
              {argument.required ? ' *' : ''}
              <input
                autoFocus
                required={argument.required}
                value={args[argument.id] || ''}
                onChange={(event) =>
                  setArgs((items) => ({ ...items, [argument.id]: event.target.value }))
                }
              />
              {argument.description && <small>{argument.description}</small>}
            </label>
          ))
        ) : (
          <p className="muted">Запрос будет подготовлен в поле ввода перед отправкой.</p>
        )}
        <details>
          <summary>Шаблон команды</summary>
          <pre className="config-preview">
            {command.content ||
              (command.promptId ? 'MCP-шаблон загружается с сервера' : command.name)}
          </pre>
        </details>
        {error && <p className="error-text">{error}</p>}
        <div className="approval-actions">
          <button type="button" onClick={close}>
            Отмена
          </button>
          <button className="primary" disabled={loading}>
            {loading ? 'Загружаем…' : 'Подготовить запрос'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
